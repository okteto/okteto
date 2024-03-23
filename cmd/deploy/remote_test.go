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
	"testing"
	"time"

	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/deployable"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeRemoteRunner struct {
	mock.Mock
}

func (f *fakeRemoteRunner) Run(ctx context.Context, params *remote.Params) error {
	args := f.Called(ctx, params)
	return args.Error(0)
}

func TestGetCommandFlags(t *testing.T) {
	type config struct {
		opts *Options
	}
	var tests = []struct {
		name      string
		config    config
		expected  []string
		expectErr bool
	}{
		{
			name: "no extra options",
			config: config{
				opts: &Options{
					Timeout: 2 * time.Minute,
					Name:    "test",
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name:    "test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name:    "this is a test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"this is a test\""},
		},
		{
			name: "namespace is not set",
			config: config{
				opts: &Options{
					Name:      "test",
					Namespace: "test",
					Timeout:   5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "manifest path is not set",
			config: config{
				opts: &Options{
					Name:             "test",
					ManifestPathFlag: "/hello/this/is/a/test",
					ManifestPath:     "/hello/this/is/a/test",
					Timeout:          5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "variables set",
			config: config{
				opts: &Options{
					Name: "test",
					Variables: []string{
						"a=b",
						"c=d",
					},
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\"", "--var a=\"b\" --var c=\"d\""},
		},
		{
			name: "wait is not set",
			config: config{
				opts: &Options{
					Name:    "test",
					Wait:    true,
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test=multi word value"},
				},
			},
			expected: []string{"--name \"test\"", "--var test=\"multi word value\""},
		},
		{
			name: "wrong multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test -> multi word value"},
				},
			},
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := GetCommandFlags(tt.config.opts.Name, tt.config.opts.Variables)
			if tt.expectErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.expected, flags)
		})
	}
}

func Test_newRemoteDeployer(t *testing.T) {
	getBuildEnvVars := func() map[string]string { return nil }
	getDependencyEnvVars := func(_ environGetter) map[string]string { return nil }
	got := newRemoteDeployer(getBuildEnvVars, io.NewIOController(), getDependencyEnvVars)
	require.IsType(t, &remoteDeployer{}, got)
	require.NotNil(t, got.getBuildEnvVars)
}

func TestDeployRemote(t *testing.T) {
	manifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
			Divert: &model.DivertDeploy{
				Namespace: "test-divert",
				Driver:    "divert-driver",
			},
			Commands: []model.DeployCommand{
				{
					Name:    "command 1",
					Command: "test-command",
				},
			},
		},
		External: map[string]*externalresource.ExternalResource{
			"test": {
				Icon: "database",
			},
		},
	}

	expectedParams := &remote.Params{
		BaseImage:           manifest.Deploy.Image,
		ManifestPathFlag:    "/path/to/manifest",
		TemplateName:        templateName,
		CommandFlags:        []string{"--name \"test\""},
		BuildEnvVars:        map[string]string{"BUILD_VAR_1": "value"},
		DependenciesEnvVars: map[string]string{"DEP_VAR_1": "value"},
		DockerfileName:      dockerfileTemporalName,
		Deployable: deployable.Entity{
			Divert:   manifest.Deploy.Divert,
			Commands: manifest.Deploy.Commands,
			External: manifest.External,
		},
		Manifest: manifest,
		Command:  remote.DeployCommand,
	}
	runner := &fakeRemoteRunner{}
	runner.On("Run", mock.Anything, expectedParams).Return(nil)
	rd := &remoteDeployer{
		runner: runner,
		getBuildEnvVars: func() map[string]string {
			return map[string]string{"BUILD_VAR_1": "value"}
		},
		getDependencyEnvVars: func(environGetter) map[string]string {
			return map[string]string{"DEP_VAR_1": "value"}
		},
	}
	opts := &Options{
		Name:             "test",
		Manifest:         manifest,
		ManifestPathFlag: "/path/to/manifest",
	}
	err := rd.Deploy(context.Background(), opts)
	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestDeployRemoteWithError(t *testing.T) {
	manifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
			Divert: &model.DivertDeploy{
				Namespace: "test-divert",
				Driver:    "divert-driver",
			},
			Commands: []model.DeployCommand{
				{
					Name:    "command 1",
					Command: "test-command",
				},
			},
		},
		External: map[string]*externalresource.ExternalResource{
			"test": {
				Icon: "database",
			},
		},
	}

	expectedParams := &remote.Params{
		BaseImage:           manifest.Deploy.Image,
		ManifestPathFlag:    "/path/to/manifest",
		TemplateName:        templateName,
		CommandFlags:        []string{"--name \"test\""},
		BuildEnvVars:        map[string]string{"BUILD_VAR_1": "value"},
		DependenciesEnvVars: map[string]string{"DEP_VAR_1": "value"},
		DockerfileName:      dockerfileTemporalName,
		Deployable: deployable.Entity{
			Divert:   manifest.Deploy.Divert,
			Commands: manifest.Deploy.Commands,
			External: manifest.External,
		},
		Manifest: manifest,
		Command:  remote.DeployCommand,
	}

	tests := []struct {
		err           error
		expectedCheck func(err error) bool
		name          string
	}{
		{
			name: "WithOktetoCommandErr",
			err: buildCmd.OktetoCommandErr{
				Stage: "test",
				Err:   assert.AnError,
			},
			expectedCheck: func(err error) bool {
				return errors.Is(err, assert.AnError)
			},
		},
		{
			name: "WithUserError",
			err: oktetoErrors.UserError{
				E: assert.AnError,
			},
			expectedCheck: func(err error) bool {
				return errors.As(err, &oktetoErrors.UserError{})
			},
		},
		{
			name: "WithOtherError",
			err:  assert.AnError,
			expectedCheck: func(err error) bool {
				return errors.Is(err, assert.AnError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRemoteRunner{}
			runner.On("Run", mock.Anything, expectedParams).Return(tt.err)
			rd := &remoteDeployer{
				runner: runner,
				getBuildEnvVars: func() map[string]string {
					return map[string]string{"BUILD_VAR_1": "value"}
				},
				getDependencyEnvVars: func(environGetter) map[string]string {
					return map[string]string{"DEP_VAR_1": "value"}
				},
			}
			opts := &Options{
				Name:             "test",
				Manifest:         manifest,
				ManifestPathFlag: "/path/to/manifest",
			}
			err := rd.Deploy(context.Background(), opts)
			require.True(t, tt.expectedCheck(err))
			runner.AssertExpectations(t)
		})
	}
}
