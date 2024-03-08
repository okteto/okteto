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
	oktetoLog "github.com/okteto/okteto/pkg/log"
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
		Namespace:    deployOptions.Manifest.Namespace,
		Variables:    deployOptions.Variables,
		ManifestPath: deployOptions.Manifest.ManifestPath,
		Deployable: deployable.Entity{
			Commands: deployOptions.Manifest.Deploy.Commands,
			Divert:   deployOptions.Manifest.Deploy.Divert,
			External: deployOptions.Manifest.External,
		},
	}

	err := ld.runner.RunDeploy(ctx, params)
	// If this returns in error, we need to set the stage to "done" as the error was already logged
	// and prevent duplication of messages when the error is handled. This is a special case when
	// the execution is happening locally. This has to be revisited when we improve the logs
	if err != nil {
		oktetoLog.SetStage("done")
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")
	}

	return err
}

func (ld *localDeployer) CleanUp(ctx context.Context, err error) {
	ld.runner.CleanUp(ctx, err)
}
