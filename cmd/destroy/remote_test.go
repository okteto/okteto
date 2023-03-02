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
	"fmt"
	"path/filepath"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	filesystem "github.com/okteto/okteto/pkg/filesystem/fake"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakeBuilder struct {
	err error
}

func (f fakeBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	return f.err
}

func (f fakeBuilder) IsV1() bool { return true }

func TestRemoteTest(t *testing.T) {
	ctx := context.Background()
	fakeManifest := &model.Manifest{
		Destroy: &model.DestroyInfo{},
	}
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	tempCreator := filesystem.NewTemporalDirectoryCtrl(fs)

	type config struct {
		wd            filesystem.FakeWorkingDirectoryCtrlErrors
		tempFsCreator error
		options       *Options
		builderErr    error
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
				options: &Options{},
			},
			expected: assert.AnError,
		},
		{
			name: "build incorrect",
			config: config{
				options:    &Options{},
				builderErr: assert.AnError,
			},
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("error during development environment deployment"),
			},
		},
		{
			name: "everything correct",
			config: config{
				options: &Options{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			tempCreator.SetError(tt.config.tempFsCreator)
			rdc := remoteDestroyCommand{
				builder:              fakeBuilder{tt.config.builderErr},
				fs:                   fs,
				workingDirectoryCtrl: wdCtrl,
				temporalCtrl:         tempCreator,
				manifest:             fakeManifest,
			}
			err := rdc.destroy(ctx, tt.config.options)
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestGetDestroyFlags(t *testing.T) {
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
				opts: &Options{},
			},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name: "test",
				},
			},
			expected: []string{"--name test"},
		},
		{
			name: "namespace set",
			config: config{
				opts: &Options{
					Namespace: "test",
				},
			},
			expected: []string{"--namespace test"},
		},
		{
			name: "manifest path set",
			config: config{
				opts: &Options{
					ManifestPathFlag: "/hello/this/is/a/test",
				},
			},
			expected: []string{"--file /hello/this/is/a/test"},
		},
		{
			name: "destroy volumes set",
			config: config{
				opts: &Options{
					DestroyVolumes: true,
				},
			},
			expected: []string{"--volumes"},
		},
		{
			name: "force destroy set",
			config: config{
				opts: &Options{
					ForceDestroy: true,
				},
			},
			expected: []string{"--force-destroy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := getDestroyFlags(tt.config.opts)
			assert.Equal(t, tt.expected, flags)
		})
	}
}

func TestCreateDockerfile(t *testing.T) {
	wdCtrl := filesystem.NewFakeWorkingDirectoryCtrl(filepath.Clean("/"))
	fs := afero.NewMemMapFs()
	fakeManifest := &model.Manifest{
		Destroy: &model.DestroyInfo{
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
				opts: &Options{},
			},
			expected: expected{
				dockerfileName: "/test/deploy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := remoteDestroyCommand{
				fs:                   fs,
				manifest:             fakeManifest,
				workingDirectoryCtrl: wdCtrl,
			}
			dockerfileName, err := rdc.createDockerfile("/test", tt.config.opts)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.dockerfileName, dockerfileName)

			if tt.expected.err == nil {
				_, err = rdc.fs.Stat(filepath.Join("/test", dockerfileTemporalNane))
				assert.NoError(t, err)
			}

		})
	}
}

func TestCreateDockerignoreIfNeeded(t *testing.T) {
	fs := afero.NewMemMapFs()

	dockerignoreWd := "/test/"
	assert.NoError(t, fs.MkdirAll(dockerignoreWd, 0755))
	assert.NoError(t, afero.WriteFile(fs, "/test/.oktetodeployignore", []byte("FROM alpine"), 0644))
	type config struct {
		wd string
	}
	var tests = []struct {
		name   string
		config config
	}{
		{
			name: "dockerignore present",
			config: config{
				wd: dockerignoreWd,
			},
		},
		{
			name:   "without dockerignore",
			config: config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdc := remoteDestroyCommand{
				fs: fs,
			}
			err := rdc.createDockerignoreIfNeeded(tt.config.wd, "/temp")
			assert.NoError(t, err)
		})
	}
}
