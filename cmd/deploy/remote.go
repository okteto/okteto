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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/spf13/afero"
)

const (
	templateName           = "dockerfile"
	dockerfileTemporalName = "Dockerfile.deploy"
)

// remoteRunner is the interface to run the deploy command remotely. The implementation is using
// an image builder like BuildKit
type remoteRunner interface {
	Run(ctx context.Context, params *remote.Params) error
}

type environGetter func() []string

type buildEnvVarsGetter func() map[string]string

type dependencyEnvVarsGetter func(environGetter environGetter) map[string]string

type remoteDeployer struct {
	getBuildEnvVars buildEnvVarsGetter
	runner          remoteRunner

	// ioCtrl is the controller for the output of the Build logs
	ioCtrl *io.Controller

	getDependencyEnvVars dependencyEnvVarsGetter
}

// newRemoteDeployer creates the remote deployer
func newRemoteDeployer(buildVarsGetter buildEnvVarsGetter, ioCtrl *io.Controller, getDependencyEnvVars dependencyEnvVarsGetter) *remoteDeployer {
	fs := afero.NewOsFs()
	builder := buildCmd.NewOktetoBuilder(
		&okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		fs,
	)
	runner := remote.NewRunner(ioCtrl, builder)
	return &remoteDeployer{
		getBuildEnvVars:      buildVarsGetter,
		runner:               runner,
		ioCtrl:               ioCtrl,
		getDependencyEnvVars: getDependencyEnvVars,
	}
}

func (rd *remoteDeployer) Deploy(ctx context.Context, deployOptions *Options) error {

	baseImage := ""
	if deployOptions.Manifest != nil && deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.Image != "" {
		baseImage = deployOptions.Manifest.Deploy.Image
	}

	dep := deployable.Entity{
		Divert:   deployOptions.Manifest.Deploy.Divert,
		Commands: deployOptions.Manifest.Deploy.Commands,
		External: deployOptions.Manifest.External,
	}

	commandsFlags, err := GetCommandFlags(deployOptions.Name, deployOptions.Variables)
	if err != nil {
		return err
	}

	runParams := remote.Params{
		// This is the base image provided by the deploy operation. If it is empty, the runner is the one in charge of
		// providing the default one
		BaseImage:           baseImage,
		ManifestPathFlag:    deployOptions.ManifestPathFlag,
		TemplateName:        templateName,
		CommandFlags:        commandsFlags,
		BuildEnvVars:        rd.getBuildEnvVars(),
		DependenciesEnvVars: rd.getDependencyEnvVars(os.Environ),
		DockerfileName:      dockerfileTemporalName,
		Deployable:          dep,
		Manifest:            deployOptions.Manifest,
		Command:             remote.DeployCommand,
	}

	if err := rd.runner.Run(ctx, &runParams); err != nil {
		var cmdErr buildCmd.OktetoCommandErr
		if errors.As(err, &cmdErr) {
			oktetoLog.SetStage(cmdErr.Stage)
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying application: %s", cmdErr.Err.Error())
			return fmt.Errorf("error deploying application: %w", cmdErr.Err)
		}
		oktetoLog.SetStage("remote deploy")
		var userErr oktetoErrors.UserError
		if errors.As(err, &userErr) {
			return userErr
		}
		return fmt.Errorf("error deploying application: %w", err)
	}

	return nil
}

func GetCommandFlags(name string, vars []string) ([]string, error) {
	var commandFlags []string
	commandFlags = append(commandFlags, fmt.Sprintf("--name %q", name))
	if len(vars) > 0 {
		var varsToAddForDeploy []string
		variables, err := env.Parse(vars)
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

func (*remoteDeployer) CleanUp(_ context.Context, _ error) {
	// Do nothing
}
