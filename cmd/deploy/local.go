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

	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
)

type runner interface {
	RunDeploy(ctx context.Context, params deployable.DeployParameters) error
	CleanUp(ctx context.Context, err error)
}

type localDeployer struct {
	runner runner
}

func newLocalDeployer(runner runner) *localDeployer {
	return &localDeployer{
		runner: runner,
	}
}

func (ld *localDeployer) Deploy(ctx context.Context, deployOptions *Options) error {
	if deployOptions.Manifest == nil {
		deployOptions.Manifest = &model.Manifest{
			Deploy:   &model.DeployInfo{},
			External: externalresource.Section{},
		}

	}
	if deployOptions.Manifest.Deploy == nil {
		deployOptions.Manifest.Deploy = &model.DeployInfo{}
	}

	params := deployable.DeployParameters{
		Name:         deployOptions.Name,
		Namespace:    deployOptions.Namespace,
		Variables:    deployOptions.Variables,
		ManifestPath: deployOptions.Manifest.ManifestPath,
		Deployable: deployable.Entity{
			Commands: deployOptions.Manifest.Deploy.Commands,
			Divert:   deployOptions.Manifest.Deploy.Divert,
			External: deployOptions.Manifest.External,
		},
	}

	// on an error, the deploy command (Command.Run) sets the "done" stage and the end of the output once the
	// dependencies have finished, so that the lines they log meanwhile are not left after the end
	return ld.runner.RunDeploy(ctx, params)
}

func (ld *localDeployer) CleanUp(ctx context.Context, err error) {
	ld.runner.CleanUp(ctx, err)
}
