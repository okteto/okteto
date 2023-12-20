// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/console"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	dockerRegistry "github.com/docker/docker/registry"
	controlapi "github.com/moby/buildkit/api/services/control"
	buildkitClient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/moby/term"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// https://github.com/docker/cli/blob/56e5910181d8ac038a634a203a4f3550bb64991f/cli/command/image/build_buildkit.go#L48
func buildWithDockerDaemonBuildkit(ctx context.Context, buildOptions *types.BuildOptions, cli *client.Client) error {
	oktetoLog.Infof("building your image with docker client v%s", cli.ClientVersion())
	s, err := session.NewSession(context.Background(), buildOptions.Path, "")
	if err != nil {
		return errors.Wrap(err, "failed to create session")
	}
	if s == nil {
		oktetoLog.Infof("buildkit not supported by daemon. Building with docker daemon")
		return buildWithDockerDaemon(ctx, buildOptions, cli)
	}

	dockerCfg := config.LoadDefaultConfigFile(os.Stderr)
	dockerAuthProvider := authprovider.NewDockerAuthProvider(dockerCfg)
	s.Allow(dockerAuthProvider)
	if len(buildOptions.Secrets) > 0 {
		secretProvider, err := build.ParseSecret(buildOptions.Secrets)
		if err != nil {
			return errors.Wrapf(err, "could not parse secrets: %v", buildOptions.Secrets)
		}
		s.Allow(secretProvider)
	}
	var (
		contextDir    string
		remote        string
		dockerfileDir string
	)

	switch {
	case isLocalDir(buildOptions.Path):
		contextDir = buildOptions.Path
		dockerfileDir = filepath.Dir(buildOptions.File)
		remote = "client-session"
	case isURL(buildOptions.Path):
		remote = buildOptions.Path
	default:
		return errors.Errorf("unable to prepare context: path %q not found", buildOptions.Path)
	}

	if dockerfileDir != "" {
		syncedDirs := filesync.StaticDirSource{
			"context":    filesync.SyncedDir{Dir: contextDir},
			"dockerfile": filesync.SyncedDir{Dir: dockerfileDir},
		}
		s.Allow(filesync.NewFSSyncProvider(syncedDirs))
	}

	eg, ctx := errgroup.WithContext(ctx)

	dialSession := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
		return cli.DialHijack(ctx, "/session", proto, meta)
	}
	eg.Go(func() error {
		return s.Run(context.TODO(), dialSession)
	})

	eg.Go(func() error {
		defer func() {
			s.Close()
		}()
		buildID := stringid.GenerateRandomID()
		dockerBuildOptions := dockerTypes.ImageBuildOptions{
			BuildID:       buildID,
			Version:       dockerTypes.BuilderBuildKit,
			Dockerfile:    filepath.Base(buildOptions.File),
			RemoteContext: remote,
			SessionID:     s.ID(),
			BuildArgs:     make(map[string]*string),
			Platform:      buildOptions.Platform,
		}
		if buildOptions.Tag != "" {
			dockerBuildOptions.Tags = append(dockerBuildOptions.Tags, buildOptions.Tag)
		}

		dockerBuildOptions.Target = buildOptions.Target

		maxArgFormatParts := 2
		for _, buildArg := range buildOptions.BuildArgs {
			kv := strings.SplitN(buildArg, "=", maxArgFormatParts)
			if len(kv) != maxArgFormatParts {
				return fmt.Errorf("invalid build-arg value %s", buildArg)
			}
			dockerBuildOptions.BuildArgs[kv[0]] = &kv[1]
		}

		response, err := cli.ImageBuild(context.Background(), nil, dockerBuildOptions)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		done := make(chan struct{})
		defer close(done)
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return cli.BuildCancel(context.TODO(), buildID)
			case <-done:
			}
			return nil
		})

		return displayStatus(eg, response, buildOptions.OutputMode, dockerAuthProvider)
	})

	return eg.Wait()
}

// https://github.com/docker/cli/blob/56e5910181d8ac038a634a203a4f3550bb64991f/cli/command/image/build.go#L209
func buildWithDockerDaemon(ctx context.Context, buildOptions *types.BuildOptions, cli *client.Client) error {
	oktetoLog.Infof("building your image with docker client v%s", cli.ClientVersion())

	dockerBuildContext, err := getBuildContext(buildOptions.File)
	if err != nil {
		return err
	}

	dockerBuildOptions, err := getDockerOptions(buildOptions)
	if err != nil {
		return err
	}
	progressOutput := streamformatter.NewProgressOutput(os.Stdout)

	var body io.Reader
	if dockerBuildContext != nil {
		body = progress.NewProgressReader(dockerBuildContext, progressOutput, 0, "", "Sending build context to Docker daemon")
	}
	res, err := cli.ImageBuild(ctx, body, dockerBuildOptions)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err != nil {
		return errors.Wrap(err, "build failed")
	}

	imageID := ""
	aux := func(msg jsonmessage.JSONMessage) {
		var result dockerTypes.BuildResult
		if err := json.Unmarshal(*msg.Aux, &result); err != nil {
			oktetoLog.Infof(fmt.Sprintf("Failed to parse aux message: %s", err))
		} else {
			imageID = result.ID
		}
	}
	termFd, isTerm := term.GetFdInfo(os.Stdout)

	err = jsonmessage.DisplayJSONMessagesStream(res.Body, os.Stdout, termFd, isTerm, aux)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			return fmt.Errorf(jerr.Message)
		}
		return err
	}
	oktetoLog.Print(imageID)
	return nil

}

func displayStatus(eg *errgroup.Group, response dockerTypes.ImageBuildResponse, buildOutputMode string, at session.Attachable) error {

	displayStatus := func(out *os.File, displayCh chan *buildkitClient.SolveStatus) {
		var c console.Console
		// TODO: Handle tty output in non-tty environment.
		if cons, err := console.ConsoleFromFile(out); err == nil && (buildOutputMode == "auto" || buildOutputMode == oktetoLog.TTYFormat) {
			c = cons
		}
		// not using shared context to not disrupt display but let it finish reporting errors
		eg.Go(func() error {
			_, err := progressui.DisplaySolveStatus(context.TODO(), "", c, out, displayCh)
			return err
		})
		if s, ok := at.(interface {
			SetLogger(progresswriter.Logger)
		}); ok {
			s.SetLogger(func(s *buildkitClient.SolveStatus) {
				displayCh <- s
			})
		}
	}
	displayCh := make(chan *buildkitClient.SolveStatus)
	displayStatus(os.Stderr, displayCh)
	defer close(displayCh)

	buf := bytes.NewBuffer(nil)

	writeAux := func(msg jsonmessage.JSONMessage) {
		if msg.ID == "moby.image.id" {
			var result dockerTypes.BuildResult
			if err := json.Unmarshal(*msg.Aux, &result); err != nil {
				oktetoLog.Errorf("failed to parse aux message: %v", err)
			}
			return
		}
		var resp controlapi.StatusResponse

		if msg.ID != "moby.buildkit.trace" {
			return
		}

		var dt []byte
		if err := json.Unmarshal(*msg.Aux, &dt); err != nil {
			return
		}
		if err := (&resp).Unmarshal(dt); err != nil {
			return
		}

		s := buildkitClient.SolveStatus{}
		for _, v := range resp.Vertexes {
			s.Vertexes = append(s.Vertexes, &buildkitClient.Vertex{
				Digest:    v.Digest,
				Inputs:    v.Inputs,
				Name:      v.Name,
				Started:   v.Started,
				Completed: v.Completed,
				Error:     v.Error,
				Cached:    v.Cached,
			})
		}
		for _, v := range resp.Statuses {
			s.Statuses = append(s.Statuses, &buildkitClient.VertexStatus{
				ID:        v.ID,
				Vertex:    v.Vertex,
				Name:      v.Name,
				Total:     v.Total,
				Current:   v.Current,
				Timestamp: v.Timestamp,
				Started:   v.Started,
				Completed: v.Completed,
			})
		}
		for _, v := range resp.Logs {
			s.Logs = append(s.Logs, &buildkitClient.VertexLog{
				Vertex:    v.Vertex,
				Stream:    int(v.Stream),
				Data:      v.Msg,
				Timestamp: v.Timestamp,
			})
		}

		displayCh <- &s
	}

	termFd, isTerm := term.GetFdInfo(os.Stdout)
	err := jsonmessage.DisplayJSONMessagesStream(response.Body, buf, termFd, isTerm, writeAux)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			return fmt.Errorf("error building image (status code %d) : %s", jerr.Code, jerr.Message)
		}
	}

	return err

}

func isLocalDir(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isURL(path string) bool {
	_, err := url.ParseRequestURI(path)
	return err == nil
}

// getBuildContext returns the build context
func getBuildContext(path string) (io.ReadCloser, error) {
	var dockerBuildContext io.ReadCloser
	var err error
	if isURL(path) {
		return nil, fmt.Errorf("Non url context is unavailable")
	} else {
		dockerBuildContext, err = createTarFromPath(path)
		if err != nil {
			return nil, err
		}
	}
	return dockerBuildContext, nil
}

// createTarFromPath creates the context tar file for docker api
func createTarFromPath(contextDir string) (io.ReadCloser, error) {
	excludes, err := readDockerignore(contextDir)
	if err != nil {
		return nil, err
	}

	build, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: excludes,
		ChownOpts:       &idtools.Identity{UID: 0, GID: 0},
	})
	if err != nil {
		return nil, err
	}
	return build, nil
}

// ReadDockerignore reads the .dockerignore file in the context directory and
// returns the list of paths to exclude
func readDockerignore(contextDir string) ([]string, error) {
	var excludes []string

	path := filepath.Join(contextDir, ".dockerignore")
	f, err := os.Open(path)
	switch {
	case os.IsNotExist(err):
		return excludes, nil
	case err != nil:
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", path, err)
		}
	}()

	return dockerignore.ReadAll(f)
}

// getDockerOptions returns the docker build options
func getDockerOptions(buildOptions *types.BuildOptions) (dockerTypes.ImageBuildOptions, error) {
	opts := dockerTypes.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Dockerfile:     buildOptions.File,
		CacheFrom:      buildOptions.CacheFrom,
		Target:         buildOptions.Target,
		NoCache:        buildOptions.NoCache,
	}
	if buildOptions.Tag != "" {
		opts.Tags = append(opts.Tags, buildOptions.Tag)
	}

	maxArgFormatParts := 2
	for _, buildArg := range buildOptions.BuildArgs {
		kv := strings.SplitN(buildArg, "=", maxArgFormatParts)
		if len(kv) != maxArgFormatParts {
			return opts, fmt.Errorf("invalid build-arg value %s", buildArg)
		}
		opts.BuildArgs[kv[0]] = &kv[1]
	}
	return opts, nil
}

func pushImage(ctx context.Context, tag string, client *client.Client) error {
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return fmt.Errorf("docker not found")
	}
	ref, err := reference.ParseNormalizedNamed(tag)
	if err != nil {
		return err
	}

	repoInfo, err := dockerRegistry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}

	authConfig := ResolveAuthConfig(ctx, dockerCli, client, repoInfo)
	if err != nil {
		return err
	}

	encodedAuth, err := registry.EncodeAuthConfig(authConfig)
	if err != nil {
		return err
	}
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(dockerCli, repoInfo.Index, "push")
	options := dockerTypes.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	responseBody, err := client.ImagePush(ctx, tag, options)
	if err != nil {
		return errors.Wrap(err, "could not push image")
	}

	return jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil)
}

func ResolveAuthConfig(ctx context.Context, dockerCli *command.DockerCli, cli *client.Client, repoInfo *dockerRegistry.RepositoryInfo) registry.AuthConfig {
	configKey := repoInfo.Index.Name
	if repoInfo.Index.Official {
		info, err := cli.Info(ctx)
		if err != nil {
			oktetoLog.Info("Error getting information about the docker server: %s", err)
		}
		configKey = info.IndexServerAddress
	}

	a, err := dockerCli.ConfigFile().GetAuthConfig(configKey)
	if err != nil {
		oktetoLog.Infof("Error getting credentials for %s: %s", configKey, err)
	}
	return registry.AuthConfig(a)
}
