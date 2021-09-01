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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/urlutil"
	dockerRegistry "github.com/docker/docker/registry"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/pkg/errors"
)

// getBuildContext returns the build context
func getBuildContext(path, dockerfilePath string) (io.ReadCloser, error) {
	var dockerBuildContext io.ReadCloser
	var err error
	if urlutil.IsURL(path) {
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

	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: excludes,
		ChownOpts:       &idtools.Identity{UID: 0, GID: 0},
	})
	if err != nil {
		return nil, err
	}
	return buildCtx, nil
}

// ReadDockerignore reads the .dockerignore file in the context directory and
// returns the list of paths to exclude
func readDockerignore(contextDir string) ([]string, error) {
	var excludes []string

	f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
	switch {
	case os.IsNotExist(err):
		return excludes, nil
	case err != nil:
		return nil, err
	}
	defer f.Close()

	return dockerignore.ReadAll(f)
}

// getDockerOptions returns the docker build options
func getDockerOptions(buildOptions BuildOptions) (types.ImageBuildOptions, error) {
	opts := types.ImageBuildOptions{
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

	for _, buildArg := range buildOptions.BuildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
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

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(dockerCli, repoInfo.Index, "push")
	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	responseBody, err := client.ImagePush(ctx, tag, options)
	if err != nil {
		return errors.Wrap(err, "could not push image")
	}

	return jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil)
}

func ResolveAuthConfig(ctx context.Context, dockerCli *command.DockerCli, cli *client.Client, repoInfo *dockerRegistry.RepositoryInfo) types.AuthConfig {
	configKey := repoInfo.Index.Name
	if repoInfo.Index.Official {
		info, err := cli.Info(ctx)
		if err != nil {
			configKey = dockerRegistry.IndexServer
		}
		if info.IndexServerAddress == "" {
			configKey = dockerRegistry.IndexServer
		}
		configKey = info.IndexServerAddress
	}

	a, _ := dockerCli.ConfigFile().GetAuthConfig(configKey)
	return types.AuthConfig(a)
}
