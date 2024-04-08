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

package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	buildV1 "github.com/okteto/okteto/cmd/build/v1"
	buildV2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd/api"
)

type fakeRegistry struct {
	registry          map[string]fakeImage
	errAddImageByOpts error
	errAddImageByName error
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
	if fr.errAddImageByName != nil {
		return fr.errAddImageByName
	}
	for _, image := range images {
		fr.registry[image] = fakeImage{}
	}
	return nil
}

func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	if fr.errAddImageByOpts != nil {
		return fr.errAddImageByOpts
	}
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
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
func (fr fakeRegistry) CloneGlobalImageToDev(_ string) (string, error) {
	return "", nil
}

var fakeManifestV2 *model.Manifest = &model.Manifest{
	Build: build.ManifestBuild{
		"test-1": &build.Info{
			Image: "test/test-1",
		},
		"test-2": &build.Info{
			Image: "test/test-2",
		},
	},
	IsV2: true,
}

func getManifestWithError(_ string, _ afero.Fs) (*model.Manifest, error) {
	return nil, assert.AnError
}

func getManifestWithInvalidManifestError(_ string, _ afero.Fs) (*model.Manifest, error) {
	return nil, oktetoErrors.ErrInvalidManifest
}

func getFakeManifestV1(_ string, _ afero.Fs) (*model.Manifest, error) {
	manifestV1 := *fakeManifestV2
	manifestV1.IsV2 = false
	return &manifestV1, nil
}

func getFakeManifestV2(_ string, _ afero.Fs) (*model.Manifest, error) {
	return fakeManifestV2, nil
}

func TestIsBuildV2(t *testing.T) {
	t.Parallel()
	tests := []struct {
		manifest       *model.Manifest
		name           string
		expectedAnswer bool
	}{
		{
			name: "manifest v1 is build v1",
			manifest: &model.Manifest{
				IsV2: false,
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v2 with no build section is build v1",
			manifest: &model.Manifest{
				IsV2:  true,
				Build: build.ManifestBuild{},
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v1 with build section is build v1",
			manifest: &model.Manifest{
				IsV2: false,
				Build: build.ManifestBuild{
					"test-1": &build.Info{
						Image: "test/test-1",
					},
					"test-2": &build.Info{
						Image: "test/test-2",
					},
				},
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v1 with build section is build v1",
			manifest: &model.Manifest{
				IsV2: false,
				Build: build.ManifestBuild{
					"test-1": &build.Info{
						Image: "test/test-1",
					},
					"test-2": &build.Info{
						Image: "test/test-2",
					},
				},
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v2 with build section is build v2",
			manifest: &model.Manifest{
				IsV2: true,
				Build: build.ManifestBuild{
					"test-1": &build.Info{
						Image: "test/test-1",
					},
					"test-2": &build.Info{
						Image: "test/test-2",
					},
				},
			},
			expectedAnswer: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer := isBuildV2(tt.manifest)
			assert.Equal(t, answer, tt.expectedAnswer)
		})
	}
}

func TestBuildIsManifestV2(t *testing.T) {
	bc := &Command{
		GetManifest: getFakeManifestV2,
	}

	manifest, err := bc.GetManifest("", afero.NewMemMapFs())
	assert.Nil(t, err)
	assert.Equal(t, manifest, fakeManifestV2)
}

func TestBuildFromDockerfile(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithError,
	}

	manifest, err := bc.GetManifest("", afero.NewMemMapFs())
	assert.NotNil(t, err)
	assert.Nil(t, manifest)
}

func TestBuildErrIfInvalidManifest(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithInvalidManifestError,
	}

	manifest, err := bc.GetManifest("", afero.NewMemMapFs())
	assert.NotNil(t, err)
	assert.Nil(t, manifest)
}

func TestBuilderIsProperlyGenerated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	okCtx := &okteto.ContextStateless{
		Store: &okteto.ContextStore{
			Contexts: map[string]*okteto.Context{
				"test": {
					Namespace: "test",
					Cfg:       &api.Config{},
				},
			},
			CurrentContext: "test",
		},
	}
	malformedDockerfile := filepath.Join(dir, "malformedDockerfile")
	dockerfile := filepath.Join(dir, "Dockerfile")
	assert.NoError(t, os.WriteFile(dockerfile, []byte(`FROM alpine`), 0600))
	assert.NoError(t, os.WriteFile(malformedDockerfile, []byte(`FROM alpine`), 0600))
	tests := []struct {
		buildCommand      *Command
		options           *types.BuildOptions
		name              string
		isBuildV2Expected bool
		expectedError     bool
	}{
		{
			name: "Manifest error fallback to v1",
			buildCommand: &Command{
				GetManifest:      getManifestWithInvalidManifestError,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options:           &types.BuildOptions{},
			expectedError:     false,
			isBuildV2Expected: false,
		},
		{
			name: "Manifest error",
			buildCommand: &Command{
				GetManifest:      getManifestWithInvalidManifestError,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options: &types.BuildOptions{
				File: "okteto.yml",
			},
			expectedError:     true,
			isBuildV2Expected: false,
		},
		{
			name: "Builder error. Dockerfile malformed",
			buildCommand: &Command{
				GetManifest:      getManifestWithInvalidManifestError,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options: &types.BuildOptions{
				File: malformedDockerfile,
			},
			expectedError:     false,
			isBuildV2Expected: false,
		},
		{
			name: "Builder error. Invalid manifest/Dockerfile correct",
			buildCommand: &Command{
				GetManifest:      getManifestWithInvalidManifestError,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options: &types.BuildOptions{
				File: dockerfile,
			},
			expectedError:     false,
			isBuildV2Expected: false,
		},
		{
			name: "BuilderV2 called.",
			buildCommand: &Command{
				GetManifest:      getFakeManifestV2,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options:           &types.BuildOptions{},
			expectedError:     false,
			isBuildV2Expected: true,
		},
		{
			name: "Manifest valid but BuilderV1 fallback.",
			buildCommand: &Command{
				GetManifest:      getFakeManifestV1,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options:           &types.BuildOptions{},
			expectedError:     false,
			isBuildV2Expected: false,
		},
		{
			name: "Manifest error. BuilderV1 fallback.",
			buildCommand: &Command{
				GetManifest:      getManifestWithError,
				Registry:         newFakeRegistry(),
				ioCtrl:           io.NewIOController(),
				analyticsTracker: fakeAnalyticsTracker{},
				insights:         fakeAnalyticsTracker{},
			},
			options:           &types.BuildOptions{},
			expectedError:     false,
			isBuildV2Expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder, err := tt.buildCommand.getBuilder(tt.options, okCtx)
			if err != nil && !tt.expectedError {
				t.Errorf("getBuilder() fail on '%s'. Expected nil error, got %s", tt.name, err.Error())
			}

			if err == nil && tt.expectedError {
				t.Errorf("getBuilder() fail on '%s'. Expected error, got nil", tt.name)
			}

			if builder == nil {
				if !tt.expectedError {
					t.Errorf("getBuilder() fail on '%s'. Expected builder, got nil", tt.name)
				}
			} else {
				switch builder.(type) {
				case *buildV1.OktetoBuilder:
					if tt.isBuildV2Expected {
						t.Errorf("getBuilder() fail on '%s'. Expected builderv2, got builderv1", tt.name)
					}
				case *buildV2.OktetoBuilder:
					if !tt.isBuildV2Expected {
						t.Errorf("getBuilder() fail on '%s'. Expected builderv1, got builderv2", tt.name)

					}
				}
			}
		})
	}

}

type fakeAnalyticsTracker struct{}

func (fakeAnalyticsTracker) TrackImageBuild(context.Context, *analytics.ImageBuildMetadata) {}

func Test_NewBuildCommand(t *testing.T) {
	okCtx := &okteto.ContextStateless{
		Store: &okteto.ContextStore{
			Contexts: map[string]*okteto.Context{
				"test": {
					Namespace: "test",
				},
			},
			CurrentContext: "test",
		},
	}
	got := NewBuildCommand(io.NewIOController(), fakeAnalyticsTracker{}, fakeAnalyticsTracker{}, okCtx, nil)
	require.IsType(t, &Command{}, got)
	require.NotNil(t, got.GetManifest)
	require.NotNil(t, got.Builder)
	require.NotNil(t, got.Registry)
	require.IsType(t, fakeAnalyticsTracker{}, got.analyticsTracker)
}

type fakeClientCfgContext struct {
	name          string
	token         string
	cert          string
	existsContext bool
}

func (c *fakeClientCfgContext) ExistsContext() bool {
	return c.existsContext
}

func (c *fakeClientCfgContext) GetCurrentName() string {
	return c.name
}

func (c *fakeClientCfgContext) GetCurrentToken() string {
	return c.token
}

func (c *fakeClientCfgContext) GetCurrentCertStr() string {
	return c.cert
}

func Test_defaultOktetoClientCfg(t *testing.T) {
	tests := []struct {
		input    *fakeClientCfgContext
		expected *okteto.ClientCfg
		name     string
	}{
		{
			name: "context not exists",
			input: &fakeClientCfgContext{
				existsContext: false,
			},
			expected: &okteto.ClientCfg{},
		},
		{
			name: "context exists",
			input: &fakeClientCfgContext{
				existsContext: true,
				name:          "test",
				token:         "okteto",
				cert:          "my-cert",
			},
			expected: &okteto.ClientCfg{
				CtxName: "test",
				Token:   "okteto",
				Cert:    "my-cert",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result := defaultOktetoClientCfg(tt.input)
			require.EqualValues(t, result, tt.expected)
		})
	}
}
