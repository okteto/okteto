// Copyright 2021 The Okteto Authors
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
	"strings"

	"github.com/containerd/console"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	controlapi "github.com/moby/buildkit/api/services/control"
	buildkitClient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/moby/term"
	"github.com/okteto/okteto/pkg/log"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func buildWithDockerDaemonBuildkit(ctx context.Context, buildOptions BuildOptions, cli *client.Client) error {
	log.Infof("building your image with docker client v%s", cli.ClientVersion())
	s, err := session.NewSession(context.Background(), buildOptions.Path, "")
	if err != nil {
		return errors.Wrap(err, "failed to create session")
	}
	if s == nil {
		log.Infof("buildkit not supported by daemon. Building with docker daemon")
		return buildWithDockerDaemon(ctx, buildOptions, cli)
	}

	dockerAuthProvider := authprovider.NewDockerAuthProvider(os.Stderr)
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
		dockerfileDir = buildOptions.Path
		remote = "client-session"
	case isURL(buildOptions.Path):
		remote = buildOptions.Path
	default:
		return errors.Errorf("unable to prepare context: path %q not found", buildOptions.Path)
	}

	if dockerfileDir != "" {
		s.Allow(filesync.NewFSSyncProvider([]filesync.SyncedDir{
			{
				Name: "context",
				Dir:  contextDir,
			},
			{
				Name: "dockerfile",
				Dir:  dockerfileDir,
			},
		}))
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
		dockerBuildOptions := types.ImageBuildOptions{
			BuildID:       buildID,
			Version:       types.BuilderBuildKit,
			Dockerfile:    buildOptions.File,
			RemoteContext: remote,
			SessionID:     s.ID(),
		}
		if buildOptions.Tag != "" {
			dockerBuildOptions.Tags = append(dockerBuildOptions.Tags, buildOptions.Tag)
		}

		for _, buildArg := range buildOptions.BuildArgs {
			kv := strings.SplitN(buildArg, "=", 2)
			if len(kv) != 2 {
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

		return displayStatus(os.Stdout, eg, response, buildOptions.OutputMode, dockerAuthProvider)
	})

	return eg.Wait()
}

func buildWithDockerDaemon(ctx context.Context, buildOptions BuildOptions, cli *client.Client) error {
	log.Infof("building your image with docker client v%s", cli.ClientVersion())

	dockerBuildContext, err := getBuildContext(buildOptions.Path, buildOptions.File)
	if err != nil {
		errors.Wrap(err, "setting build context failed")
	}

	dockerBuildOptions, err := getDockerOptions(buildOptions)
	if err != nil {
		errors.Wrap(err, "setting build options failed")
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
		var result types.BuildResult
		if err := json.Unmarshal(*msg.Aux, &result); err != nil {
			log.Infof(fmt.Sprintf("Failed to parse aux message: %s", err))
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
	fmt.Print(imageID)
	return nil

}

func displayStatus(out *os.File, eg *errgroup.Group, response types.ImageBuildResponse, buildOutputMode string, at session.Attachable) error {

	displayStatus := func(out *os.File, displayCh chan *buildkitClient.SolveStatus) {
		var c console.Console
		// TODO: Handle tty output in non-tty environment.
		if cons, err := console.ConsoleFromFile(out); err == nil && (buildOutputMode == "auto" || buildOutputMode == "tty") {
			c = cons
		}
		// not using shared context to not disrupt display but let it finish reporting errors
		eg.Go(func() error {
			return progressui.DisplaySolveStatus(context.TODO(), "", c, out, displayCh)
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
			var result types.BuildResult
			if err := json.Unmarshal(*msg.Aux, &result); err != nil {
				log.Errorf("failed to parse aux message: %v", err)
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
