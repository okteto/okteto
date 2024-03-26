// Copyright 2024 The Okteto Authors
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

package remoterun

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeDeployRunner struct {
	mock.Mock
}

func (f *fakeDeployRunner) RunDeploy(ctx context.Context, params deployable.DeployParameters) error {
	args := f.Called(ctx, params)
	return args.Error(0)
}

func TestRun(t *testing.T) {
	params := deployable.DeployParameters{
		Name:      "test",
		Namespace: "ns",
		Deployable: deployable.Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "Command 1",
					Command: "echo 1",
				},
				{
					Name:    "Command 2",
					Command: "echo 2",
				},
			},
			External: externalresource.Section{
				"fake": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
			Divert: &model.DivertDeploy{
				Driver:    "test driver",
				Namespace: "ns",
			},
		},
	}
	runner := &fakeDeployRunner{}

	c := &DeployCommand{
		runner: runner,
	}

	runner.On("RunDeploy", mock.Anything, params).Return(nil)

	err := c.Run(context.Background(), params)

	assert.NoError(t, err)
	runner.AssertExpectations(t)
}
