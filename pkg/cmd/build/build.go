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
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/pkg/errors"
)

//BuildOptions define the options available for build
type BuildOptions struct {
	BuildArgs  []string
	CacheFrom  []string
	File       string
	NoCache    bool
	OutputMode string
	Path       string
	Secrets    []string
	Tag        string
	Target     string
	Namespace  string
}

// Run runs the build sequence
func Run(ctx context.Context, buildOptions BuildOptions) error {
	buildOptions.OutputMode = setOutputMode(buildOptions.OutputMode)
	if okteto.Context().Builder == "" {
		if err := buildWithDocker(ctx, buildOptions); err != nil {
			return err
		}
	} else {
		if err := buildWithOkteto(ctx, buildOptions); err != nil {
			return err
		}
	}
	return nil
}

func setOutputMode(outputMode string) string {
	if buildOutput := os.Getenv(model.BuildkitProgressEnvVar); buildOutput != "" {
		return buildOutput
	}
	if utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
		return "plain"
	}
	return outputMode
}

func buildWithOkteto(ctx context.Context, buildOptions BuildOptions) error {
	oktetoLog.Infof("building your image on %s", okteto.Context().Builder)
	buildkitClient, err := getBuildkitClient(ctx)
	if err != nil {
		return err
	}

	if buildOptions.File != "" {
		buildOptions.File, err = registry.GetDockerfile(buildOptions.File)
		if err != nil {
			return err
		}
		defer os.Remove(buildOptions.File)
	}

	if buildOptions.Tag != "" {
		err = validateImage(buildOptions.Tag)
		if err != nil {
			return err
		}
	}

	if okteto.IsOkteto() {
		buildOptions.Tag = registry.ExpandOktetoDevRegistry(buildOptions.Tag)
		buildOptions.Tag = registry.ExpandOktetoGlobalRegistry(buildOptions.Tag)
		for i := range buildOptions.CacheFrom {
			buildOptions.CacheFrom[i] = registry.ExpandOktetoDevRegistry(buildOptions.CacheFrom[i])
			buildOptions.CacheFrom[i] = registry.ExpandOktetoGlobalRegistry(buildOptions.CacheFrom[i])
		}
	}
	opt, err := getSolveOpt(buildOptions)
	if err != nil {
		return errors.Wrap(err, "failed to create build solver")
	}

	err = solveBuild(ctx, buildkitClient, opt, buildOptions.OutputMode)
	if err != nil {
		oktetoLog.Infof("Failed to build image: %s", err.Error())
	}
	if registry.IsTransientError(err) {
		oktetoLog.Yellow(`Failed to push '%s' to the registry:
  %s,
  Retrying ...`, buildOptions.Tag, err.Error())
		success := true
		err := solveBuild(ctx, buildkitClient, opt, buildOptions.OutputMode)
		if err != nil {
			success = false
			oktetoLog.Infof("Failed to build image: %s", err.Error())
		}
		err = registry.GetErrorMessage(err, buildOptions.Tag)
		analytics.TrackBuildTransientError(okteto.Context().Builder, success)
		return err
	}

	if buildOptions.Tag != "" {
		if _, err := registry.GetImageTagWithDigest(buildOptions.Tag); err != nil {
			oktetoLog.Yellow(`Failed to push '%s' metadata to the registry:
	  %s,
	  Retrying ...`, buildOptions.Tag, err.Error())
			success := true
			err := solveBuild(ctx, buildkitClient, opt, buildOptions.OutputMode)
			if err != nil {
				success = false
				oktetoLog.Infof("Failed to build image: %s", err.Error())
			}
			err = registry.GetErrorMessage(err, buildOptions.Tag)
			analytics.TrackBuildPullError(okteto.Context().Builder, success)
			return err
		}
	}

	err = registry.GetErrorMessage(err, buildOptions.Tag)
	return err
}

// https://github.com/docker/cli/blob/56e5910181d8ac038a634a203a4f3550bb64991f/cli/command/image/build.go#L209
func buildWithDocker(ctx context.Context, buildOptions BuildOptions) error {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	if versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.39") {
		err = buildWithDockerDaemonBuildkit(ctx, buildOptions, cli)
		if err != nil {
			return translateDockerErr(err)
		}
	} else {
		err = buildWithDockerDaemon(ctx, buildOptions, cli)
		if err != nil {
			return translateDockerErr(err)
		}
	}
	if buildOptions.Tag != "" {
		return pushImage(ctx, buildOptions.Tag, cli)
	}
	return nil
}

func validateImage(imageTag string) error {
	if (registry.IsOktetoRegistry(imageTag)) && strings.Count(imageTag, "/") != 1 {
		prefix := okteto.DevRegistry
		if registry.IsGlobalRegistry(imageTag) {
			prefix = okteto.GlobalRegistry
		}
		return oktetoErrors.UserError{
			E:    fmt.Errorf("Can not use '%s' as the image tag.", imageTag),
			Hint: fmt.Sprintf("The syntax for using okteto registry is: '%s/image_name'", prefix),
		}
	}
	return nil
}

func translateDockerErr(err error) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), "failed to dial gRPC: cannot connect to the Docker daemon") {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("cannot connect to Docker Daemon"),
			Hint: "Please start the service and try again",
		}
	}
	return err
}

func OptsFromManifest(service string, b *model.BuildInfo, o BuildOptions) BuildOptions {
	if okteto.Context().IsOkteto && b.Image == "" {
		b.Image = fmt.Sprintf("%s/%s:%s", okteto.DevRegistry, service, "dev")
	}

	opts := BuildOptions{
		CacheFrom: b.CacheFrom,
		Target:    b.Target,
		Path:      b.Context,
		Tag:       b.Image,
		File:      filepath.Join(b.Context, b.Dockerfile),
	}

	if len(b.Args) != 0 {
		opts.BuildArgs = model.SerializeBuildArgs(b.Args)
	}

	opts.OutputMode = setOutputMode(o.OutputMode)
	return opts
}
