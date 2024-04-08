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
	"testing"

	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/deployable"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource"
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
		name     string
		config   config
		expected []string
	}{
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name: "test",
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name: "this is a test",
				},
			},
			expected: []string{"--name \"this is a test\""},
		},
		{
			name: "force destroy set",
			config: config{
				opts: &Options{
					Name:         "test",
					ForceDestroy: true,
				},
			},
			expected: []string{"--name \"test\"", "--force-destroy"},
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
				},
			},
			expected: []string{"--name \"test\"", "--var a=\"b\" --var c=\"d\""},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := getCommandFlags(tt.config.opts)
			assert.Equal(t, tt.expected, flags)
			assert.NoError(t, err)
		})
	}
}

func TestDestroyRemote(t *testing.T) {
	manifest := &model.Manifest{
		Destroy: &model.DestroyInfo{
			Image: "test-image",
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
		BaseImage:        manifest.Destroy.Image,
		ManifestPathFlag: "/path/to/manifest",
		TemplateName:     templateName,
		CommandFlags:     []string{"--name \"test\""},
		DockerfileName:   dockerfileTemporalName,
		Deployable: deployable.Entity{
			Commands: manifest.Destroy.Commands,
			External: manifest.External,
		},
		BuildEnvVars:        make(map[string]string),
		DependenciesEnvVars: make(map[string]string),
		Manifest:            manifest,
		Command:             remote.DestroyCommand,
	}
	runner := &fakeRemoteRunner{}
	runner.On("Run", mock.Anything, expectedParams).Return(nil)
	rd := &remoteDestroyCommand{
		runner: runner,
	}
	opts := &Options{
		Name:             "test",
		Manifest:         manifest,
		ManifestPathFlag: "/path/to/manifest",
	}
	err := rd.Destroy(context.Background(), opts)
	require.NoError(t, err)
	runner.AssertExpectations(t)
}

func TestDestroyRemoteWithError(t *testing.T) {
	manifest := &model.Manifest{
		Destroy: &model.DestroyInfo{
			Image: "test-image",
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
		BaseImage:        manifest.Destroy.Image,
		ManifestPathFlag: "/path/to/manifest",
		TemplateName:     templateName,
		CommandFlags:     []string{"--name \"test\""},
		DockerfileName:   dockerfileTemporalName,
		Deployable: deployable.Entity{
			Commands: manifest.Destroy.Commands,
			External: manifest.External,
		},
		BuildEnvVars:        make(map[string]string),
		DependenciesEnvVars: make(map[string]string),
		Manifest:            manifest,
		Command:             remote.DestroyCommand,
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
			rd := &remoteDestroyCommand{
				runner: runner,
			}
			opts := &Options{
				Name:             "test",
				Manifest:         manifest,
				ManifestPathFlag: "/path/to/manifest",
			}
			err := rd.Destroy(context.Background(), opts)
			require.True(t, tt.expectedCheck(err))
			runner.AssertExpectations(t)
		})
	}
}
