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
	"path/filepath"
	"testing"
	"time"

	v2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	filesystem "github.com/okteto/okteto/pkg/filesystem/fake"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBuilder struct {
	err error
}

func (f fakeBuilder) Build(_ context.Context, _ *types.BuildOptions) error {
	return f.err
}

func (f fakeBuilder) IsV1() bool { return true }

func TestRemoteTest(t *testing.T) {
	ctx := context.Background()
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
		},
	}
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	tempCreator := filesystem.NewTemporalDirectoryCtrl(fs)

	type config struct {
		wd            filesystem.FakeWorkingDirectoryCtrlErrors
		tempFsCreator error
		options       *Options
		builderErr    error
		cert          []byte
	}
	var tests = []struct {
		name     string
		config   config
		expected error
	}{
		{
			name: "OS can't access to the working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Getter: assert.AnError,
				},
				options: &Options{},
				cert:    []byte("this-is-my-cert-there-are-many-like-it-but-this-one-is-mine"),
			},
			expected: assert.AnError,
		},
		{
			name: "OS can't create temporal directory",
			config: config{
				options:       &Options{},
				tempFsCreator: assert.AnError,
			},
			expected: assert.AnError,
		},
		{
			name: "OS can't change to the previous working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Setter: assert.AnError,
				},
				options: &Options{
					Manifest: fakeManifest,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "build incorrect",
			config: config{
				options: &Options{
					Manifest: fakeManifest,
				},
				builderErr: assert.AnError,
			},
			expected: oktetoErrors.UserError{
				E: assert.AnError,
			},
		},
		{
			name: "build with command error",
			config: config{
				options: &Options{
					Manifest: fakeManifest,
				},
				builderErr: build.OktetoCommandErr{
					Stage: "test",
					Err:   assert.AnError,
				},
			},
			expected: oktetoErrors.UserError{
				E: assert.AnError,
			},
		},
		{
			name: "everything correct",
			config: config{
				options: &Options{
					Manifest: fakeManifest,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			tempCreator.SetError(tt.config.tempFsCreator)
			rdc := remoteDeployCommand{
				builderV2: &v2.OktetoBuilder{
					Registry: newFakeRegistry(),
				},
				builderV1:            fakeBuilder{tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				clusterMetadata: func(context.Context) (*types.ClusterMetadata, error) {
					return &types.ClusterMetadata{Certificate: tt.config.cert}, nil
				},
			}
			err := rdc.deploy(ctx, tt.config.options)
			if tt.expected != nil {
				assert.EqualError(t, err, tt.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDeployFlags(t *testing.T) {
	type config struct {
		opts *Options
	}
	var tests = []struct {
		name     string
		config   config
		expected []string
	}{
		{
			name: "no extra options",
			config: config{
				opts: &Options{
					Timeout: 2 * time.Minute,
				},
			},
			expected: []string{"--timeout 2m0s"},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name:    "test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\"", "--timeout 5m0s"},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name:    "this is a test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"this is a test\"", "--timeout 5m0s"},
		},
		{
			name: "namespace set",
			config: config{
				opts: &Options{
					Namespace: "test",
					Timeout:   5 * time.Minute,
				},
			},
			expected: []string{"--namespace test", "--timeout 5m0s"},
		},
		{
			name: "manifest path set",
			config: config{
				opts: &Options{
					ManifestPathFlag: "/hello/this/is/a/test",
					Timeout:          5 * time.Minute,
				},
			},
			expected: []string{"--file /hello/this/is/a/test", "--timeout 5m0s"},
		},
		{
			name: "variables set",
			config: config{
				opts: &Options{
					Variables: []string{
						"a=b",
						"c=d",
					},
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--var a=b --var c=d", "--timeout 5m0s"},
		},
		{
			name: "wait set",
			config: config{
				opts: &Options{
					Wait:    true,
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--wait", "--timeout 5m0s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := getDeployFlags(tt.config.opts)
			assert.Equal(t, tt.expected, flags)
		})
	}
}

func TestCreateDockerfile(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{
			Image: "test-image",
		},
	}
	type config struct {
		wd   filesystem.FakeWorkingDirectoryCtrlErrors
		opts *Options
	}
	type expected struct {
		dockerfileName string
		err            error
	}
	var tests = []struct {
		name     string
		config   config
		expected expected
	}{
		{
			name: "OS can't access working directory",
			config: config{
				wd: filesystem.FakeWorkingDirectoryCtrlErrors{
					Getter: assert.AnError,
				},
			},
			expected: expected{
				dockerfileName: "",
				err:            assert.AnError,
			},
		},
		{
			name: "with dockerignore",
			config: config{
				opts: &Options{
					Manifest: fakeManifest,
				},
			},
			expected: expected{
				dockerfileName: filepath.Clean("/test/Dockerfile.deploy"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := remoteDeployCommand{
				builderV2:            &v2.OktetoBuilder{},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
			}
			dockerfileName, err := rdc.createDockerfile("/test", tt.config.opts)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.dockerfileName, dockerfileName)

			if tt.expected.err == nil {
				_, err = rdc.fs.Stat(filepath.Join("/test", dockerfileTemporalName))
				assert.NoError(t, err)
			}

		})
	}
}

func TestCreateDockerignore(t *testing.T) {
	fs := afero.NewMemMapFs()
	tempDir := "/temp"

	dockerignoreWd := "/test/"
	assert.NoError(t, fs.MkdirAll(dockerignoreWd, 0755))
	assert.NoError(t, afero.WriteFile(fs, filepath.Join(dockerignoreWd, ".oktetodeployignore"), []byte("FROM alpine"), 0644))
	type config struct {
		wd string
	}
	var tests = []struct {
		name            string
		config          config
		expectedContent string
	}{
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "FROM alpine",
		},
		{
			name:            "without dockerignore generate empty dockerignore",
			config:          config{},
			expectedContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc := remoteDeployCommand{
				fs: fs,
			}
			err := rdc.createDockerignore(tt.config.wd, tempDir)
			b, _ := afero.ReadFile(rdc.fs, filepath.Join(tempDir, ".dockerignore"))
			assert.Equal(t, tt.expectedContent, string(b))
			assert.NoError(t, err)
		})
	}
}

func Test_getOktetoCLIVersion(t *testing.T) {
	var tests = []struct {
		name                                 string
		versionString, expected, cliImageEnv string
	}{
		{
			name:          "no version string and no env return latest",
			versionString: "",
			expected:      "okteto/okteto:latest",
		},
		{
			name:          "no version string return env value",
			versionString: "",
			cliImageEnv:   "okteto/remote:test",
			expected:      "okteto/remote:test",
		},
		{
			name:          "found version string",
			versionString: "2.2.2",
			expected:      "okteto/okteto:2.2.2",
		},
		{
			name:          "found incorrect version string return latest ",
			versionString: "2.a.2",
			expected:      "okteto/okteto:latest",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.cliImageEnv != "" {
				t.Setenv(constants.OktetoDeployRemoteImage, tt.cliImageEnv)
			}

			version := getOktetoCLIVersion(tt.versionString)
			require.Equal(t, version, tt.expected)
		})
	}
}
