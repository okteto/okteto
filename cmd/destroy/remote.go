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

package destroy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/spf13/afero"
)

const (
	templateName           = "destroy-dockerfile"
	dockerfileTemporalName = "Dockerfile.destroy"
)

// remoteRunner is the interface to run the destroy command remotely. The implementation is using
// an image builder like BuildKit
type remoteRunner interface {
	Run(ctx context.Context, params *remote.Params) error
}

type remoteDestroyCommand struct {
	runner   remoteRunner
	manifest *model.Manifest

	// ioCtrl is the controller for the output of the Build logs
	ioCtrl *io.Controller
}

// newRemoteDestroyer creates a new remote destroyer
func newRemoteDestroyer(manifest *model.Manifest, ioCtrl *io.Controller) *remoteDestroyCommand {
	fs := afero.NewOsFs()
	builder := buildCmd.NewOktetoBuilder(
		&okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		fs,
	)
	runner := remote.NewRunner(ioCtrl, builder)
	if manifest.Destroy == nil {
		manifest.Destroy = &model.DestroyInfo{}
	}
	return &remoteDestroyCommand{
		runner:   runner,
		manifest: manifest,
		ioCtrl:   ioCtrl,
	}
}

func (rd *remoteDestroyCommand) Destroy(ctx context.Context, opts *Options) error {
	if opts.Manifest == nil {
		opts.Manifest = &model.Manifest{}
	}

	if opts.Manifest.Destroy == nil {
		opts.Manifest.Destroy = &model.DestroyInfo{}
	}

	baseImage := ""
	if opts.Manifest.Destroy.Image != "" {
		baseImage = opts.Manifest.Destroy.Image
	}

	dep := deployable.Entity{
		Commands: opts.Manifest.Destroy.Commands,
		// Added this for backward compatibility. Before the refactor we were having the env variables for the external
		// resources in the environment, so including it to set the env vars in the remote-run
		External: opts.Manifest.External,
	}

	commandFlags, err := getCommandFlags(opts)
	if err != nil {
		return err
	}

	runParams := remote.Params{
		BaseImage:           baseImage,
		ManifestPathFlag:    opts.ManifestPathFlag,
		TemplateName:        templateName,
		CommandFlags:        commandFlags,
		BuildEnvVars:        make(map[string]string),
		DependenciesEnvVars: make(map[string]string),
		DockerfileName:      dockerfileTemporalName,
		Deployable:          dep,
		Manifest:            opts.Manifest,
		Command:             remote.DestroyCommand,
	}

	// we need to call Run() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	if err := rd.runner.Run(ctx, &runParams); err != nil {
		var cmdErr buildCmd.OktetoCommandErr
		if errors.As(err, &cmdErr) {
			oktetoLog.SetStage(cmdErr.Stage)
			return oktetoErrors.UserError{
				E: fmt.Errorf("error during development environment deployment: %w", cmdErr.Err),
			}
		}
		oktetoLog.SetStage("remote deploy")
		var userErr oktetoErrors.UserError
		if errors.As(err, &userErr) {
			return userErr
		}
		return oktetoErrors.UserError{
			E: fmt.Errorf("error during destroy of the development environment: %w", err),
		}
	}
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return nil
}

func getCommandFlags(opts *Options) ([]string, error) {
	commandFlags := []string{
		fmt.Sprintf("--name \"%s\"", opts.Name),
	}

	if opts.ForceDestroy {
		commandFlags = append(commandFlags, "--force-destroy")
	}

	if len(opts.Variables) > 0 {
		var varsToAddForDeploy []string
		variables, err := env.Parse(opts.Variables)
		if err != nil {
			return nil, err
		}
		for _, v := range variables {
			varsToAddForDeploy = append(varsToAddForDeploy, fmt.Sprintf("--var %s=%q", v.Name, v.Value))
		}
		commandFlags = append(commandFlags, strings.Join(varsToAddForDeploy, " "))
	}

	return commandFlags, nil
}
