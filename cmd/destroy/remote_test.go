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
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	filesystem "github.com/okteto/okteto/pkg/filesystem/fake"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/registry"
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

func (fakeBuilder) IsV1() bool { return true }

type fakeRegistry struct {
	err      error
	registry map[string]fakeImage
}

// fakeImage represents the data from an image
type fakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

func newFakeRegistry() fakeRegistry {
	return fakeRegistry{
		registry: map[string]fakeImage{},
	}
}

func (fr fakeRegistry) HasGlobalPushAccess() (bool, error) { return false, nil }

func (fr fakeRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fr.registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}
func (fr fakeRegistry) IsOktetoRegistry(_ string) bool { return false }

func (fr fakeRegistry) AddImageByName(images ...string) error {
	for _, image := range images {
		fr.registry[image] = fakeImage{}
	}
	return nil
}
func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
}
func (fr fakeRegistry) getFakeImage(image string) fakeImage {
	v, ok := fr.registry[image]
	if ok {
		return v
	}
	return fakeImage{}
}
func (fr fakeRegistry) GetImageReference(image string) (registry.OktetoImageReference, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return registry.OktetoImageReference{}, err
	}
	return registry.OktetoImageReference{
		Registry: ref.Context().RegistryStr(),
		Repo:     ref.Context().RepositoryStr(),
		Tag:      ref.Identifier(),
		Image:    image,
	}, nil
}

func (fr fakeRegistry) IsGlobalRegistry(image string) bool { return false }

func (fr fakeRegistry) GetRegistryAndRepo(image string) (string, string) { return "", "" }
func (fr fakeRegistry) GetRepoNameAndTag(repo string) (string, string)   { return "", "" }

func TestRemoteTest(t *testing.T) {
	ctx := context.Background()
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
				E: fmt.Errorf("error during destroy of the development environment: %w", assert.AnError),
			},
		},
		{
			name: "build with command error",
			config: config{
				options: &Options{},
				builderErr: build.OktetoCommandErr{
					Stage: "test",
					Err:   assert.AnError,
				},
			},
			expected: oktetoErrors.UserError{
				E: fmt.Errorf("error during development environment deployment: %w", assert.AnError),
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
				destroyImage:         "",
				registry:             newFakeRegistry(),
				certFetcher: func(ctx context.Context) ([]byte, error) {
					return []byte("cert"), nil
				},
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
	type config struct {
		wd   filesystem.FakeWorkingDirectoryCtrlErrors
		opts *Options
	}
	type expected struct {
		dockerfileName string
		err            error
	}
	var tests = []struct {
		name            string
		config          config
		expected        expected
		actionNameValue string
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
				dockerfileName: filepath.Clean("/test/deploy"),
			},
			actionNameValue: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wdCtrl.SetErrors(tt.config.wd)
			rdc := remoteDestroyCommand{
				fs:                   fs,
				destroyImage:         "test-image",
				workingDirectoryCtrl: wdCtrl,
				registry:             newFakeRegistry(),
			}
			t.Setenv(model.OktetoActionNameEnvVar, tt.actionNameValue)
			dockerfileName, err := rdc.createDockerfile("/test", tt.config.opts)
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.dockerfileName, dockerfileName)

			if tt.expected.err == nil {
				_, err = rdc.fs.Stat(filepath.Join("/test", dockerfileTemporalNane))
				assert.NoError(t, err)
				content, _ := afero.ReadFile(rdc.fs, filepath.Join("/test", dockerfileTemporalNane))
				assert.True(t, strings.Contains(string(content), fmt.Sprintf("ENV %s %s", model.OktetoActionNameEnvVar, tt.actionNameValue)))
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
				fs:       fs,
				registry: newFakeRegistry(),
			}
			err := rdc.createDockerignoreIfNeeded(tt.config.wd, "/temp")
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
				t.Setenv(constants.OKtetoDeployRemoteImage, tt.cliImageEnv)
			}

			version := getOktetoCLIVersion(tt.versionString)
			require.Equal(t, version, tt.expected)
		})
	}
}
