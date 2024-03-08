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
	"testing"

	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	mock.Mock
}

func (f *fakeRunner) RunDeploy(ctx context.Context, params deployable.DeployParameters) error {
	args := f.Called(ctx, params)
	return args.Error(0)
}

func (f *fakeRunner) CleanUp(ctx context.Context, err error) {
	f.Called(ctx, err)
}

func TestDeploy(t *testing.T) {
	r := &fakeRunner{}
	opts := &Options{
		Name: "test",
		Manifest: &model.Manifest{
			Namespace: "ns",
			Deploy: &model.DeployInfo{
				Commands: []model.DeployCommand{
					{
						Name:    "test command",
						Command: "echo",
					},
				},
				Divert: &model.DivertDeploy{
					Namespace: "ns",
					Driver:    "test driver",
				},
			},
			ManifestPath: "path to manifest",
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
		Variables: []string{
			"A=value1",
			"B=value2",
		},
	}

	expectedParams := deployable.DeployParameters{
		Name:      "test",
		Namespace: "ns",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		ManifestPath: "path to manifest",
		Deployable: deployable.Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "test command",
					Command: "echo",
				},
			},
			Divert: &model.DivertDeploy{
				Namespace: "ns",
				Driver:    "test driver",
			},
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
	}
	r.On("RunDeploy", mock.Anything, expectedParams).Return(assert.AnError)

	d := newLocalDeployer(r)
	err := d.Deploy(context.Background(), opts)

	require.ErrorIs(t, err, assert.AnError)
	r.AssertExpectations(t)
}

func TestDeployWithEmptyManifest(t *testing.T) {
	r := &fakeRunner{}
	opts := &Options{
		Name: "test",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
	}

	expectedParams := deployable.DeployParameters{
		Name: "test",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		Deployable: deployable.Entity{
			External: externalresource.Section{},
		},
	}
	r.On("RunDeploy", mock.Anything, expectedParams).Return(assert.AnError)

	d := newLocalDeployer(r)
	err := d.Deploy(context.Background(), opts)

	require.ErrorIs(t, err, assert.AnError)
	r.AssertExpectations(t)
}

func TestDeployWithEmptyDeploySection(t *testing.T) {
	r := &fakeRunner{}
	opts := &Options{
		Name: "test",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		Manifest: &model.Manifest{
			Namespace:    "ns",
			ManifestPath: "path to manifest",
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
	}

	expectedParams := deployable.DeployParameters{
		Name:      "test",
		Namespace: "ns",
		Variables: []string{
			"A=value1",
			"B=value2",
		},
		ManifestPath: "path to manifest",
		Deployable: deployable.Entity{
			External: externalresource.Section{
				"db": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
		},
	}
	r.On("RunDeploy", mock.Anything, expectedParams).Return(assert.AnError)

	d := newLocalDeployer(r)
	err := d.Deploy(context.Background(), opts)

	require.ErrorIs(t, err, assert.AnError)
	r.AssertExpectations(t)
}

func TestCleanUp(t *testing.T) {
	r := &fakeRunner{}
	r.On("CleanUp", mock.Anything, assert.AnError)

	d := newLocalDeployer(r)
	d.CleanUp(context.Background(), assert.AnError)

	r.AssertExpectations(t)
}
