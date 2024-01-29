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

package v2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/okteto/okteto/cmd/build/v1"
	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	"github.com/okteto/okteto/internal/test"
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
)

var fakeManifest *model.Manifest = &model.Manifest{
	Name: "test",
	Build: build.ManifestBuild{
		"test-1": &build.Info{
			Image:      "test/test-1",
			Context:    ".",
			Dockerfile: ".",
		},
		"test-2": &build.Info{
			Image:      "test/test-2",
			Context:    ".",
			Dockerfile: ".",
			VolumesToInclude: []build.VolumeMounts{
				{
					LocalPath:  "/tmp",
					RemotePath: "/tmp",
				},
			},
		},
		"test-3": &build.Info{
			Context:    ".",
			Dockerfile: ".",
		},
		"test-4": &build.Info{
			Context:    ".",
			Dockerfile: ".",
			VolumesToInclude: []build.VolumeMounts{
				{
					LocalPath:  "/tmp",
					RemotePath: "/tmp",
				},
			},
		},
	},
	IsV2: true,
}

type fakeRegistry struct {
	registry          map[string]fakeImage
	errAddImageByName error
	errAddImageByOpts error
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
func (fr fakeRegistry) CloneGlobalImageToDev(imageWithDigest, tag string) (string, error) {
	return "", nil
}

type fakeAnalyticsTracker struct {
	metaPayload []*analytics.ImageBuildMetadata
}

func (a *fakeAnalyticsTracker) TrackImageBuild(meta ...*analytics.ImageBuildMetadata) {
	a.metaPayload = meta
}

func NewFakeBuilder(builder OktetoBuilderInterface, registry oktetoRegistryInterface, cfg oktetoBuilderConfigInterface, analyticsTracker analyticsTrackerInterface) *OktetoBuilder {
	return &OktetoBuilder{
		Registry:          registry,
		Builder:           builder,
		buildEnvironments: make(map[string]string),
		V1Builder: &v1.OktetoBuilder{
			Builder:  builder,
			Registry: registry,
			IoCtrl:   io.NewIOController(),
		},
		Config:           cfg,
		ioCtrl:           io.NewIOController(),
		analyticsTracker: analyticsTracker,
		smartBuildCtrl:   smartbuild.NewSmartBuildCtrl(fakeConfigRepo{}, registry, afero.NewMemMapFs(), io.NewIOController()),
		oktetoContext: &okteto.ContextStateless{
			Store: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"test": {
						Namespace: "test",
						IsOkteto:  true,
						Registry:  "my-registry",
					},
				},
				CurrentContext: "test",
			},
		},
	}
}

func TestValidateOptions(t *testing.T) {
	var tests = []struct {
		name         string
		buildSection build.ManifestBuild
		svcsToBuild  []string
		options      types.BuildOptions
		expectedErr  bool
	}{
		{
			name:         "no services to build",
			buildSection: build.ManifestBuild{},
			svcsToBuild:  []string{},
			options:      types.BuildOptions{},
			expectedErr:  true,
		},
		{
			name:         "svc not defined on manifest build section",
			buildSection: build.ManifestBuild{},
			svcsToBuild:  []string{"test"},
			options:      types.BuildOptions{},
			expectedErr:  true,
		},
		{
			name: "several services but with flag",
			buildSection: build.ManifestBuild{
				"test":   &build.Info{},
				"test-2": &build.Info{},
			},
			svcsToBuild: []string{"test", "test-2"},
			options: types.BuildOptions{
				Tag: "test",
			},
			expectedErr: true,
		},
		{
			name: "only one service without flags",
			buildSection: build.ManifestBuild{
				"test": &build.Info{},
			},
			svcsToBuild: []string{"test"},
			options:     types.BuildOptions{},
			expectedErr: false,
		},
		{
			name: "only one service with flags",
			buildSection: build.ManifestBuild{
				"test": &build.Info{},
			},
			svcsToBuild: []string{"test"},
			options: types.BuildOptions{
				Tag: "test",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &model.Manifest{Build: tt.buildSection}
			err := validateOptions(manifest, tt.svcsToBuild, &tt.options)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOnlyInjectVolumeMountsInOkteto(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"test": &build.Info{
				Image: "nginx",
				VolumesToInclude: []build.VolumeMounts{
					{
						LocalPath:  dir,
						RemotePath: "test",
					},
				},
			},
		},
	}
	image, err := bc.buildServiceImages(ctx, manifest, "test", &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto.dev/test-test:okteto-with-volume-mounts", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestTwoStepsBuild(t *testing.T) {
	ctx := context.Background()

	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"test": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				VolumesToInclude: []build.VolumeMounts{
					{
						LocalPath:  dir,
						RemotePath: "test",
					},
				},
			},
		},
	}
	image, err := bc.buildServiceImages(ctx, manifest, "test", &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto.dev/test-test:okteto-with-volume-mounts", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
	image, err = bc.Registry.GetImageTagWithDigest("okteto.dev/test-test:okteto")
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestBuildWithoutVolumeMountWithoutImage(t *testing.T) {
	ctx := context.Background()

	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"test": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
			},
		},
	}
	image, err := bc.buildServiceImages(ctx, manifest, "test", &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto.dev/test-test:okteto", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestBuildWithoutVolumeMountWithImage(t *testing.T) {
	ctx := context.Background()

	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"test": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Image:      "okteto/test",
			},
		},
	}
	image, err := bc.buildServiceImages(ctx, manifest, "test", &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto/test", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestBuildWithStack(t *testing.T) {
	ctx := context.Background()

	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	manifest := &model.Manifest{
		Name: "test",
		Type: model.StackType,
		Build: build.ManifestBuild{
			"test": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Image:      "okteto/test:q",
			},
		},
	}
	image, err := bc.buildServiceImages(ctx, manifest, "test", &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto.dev/test-test:okteto", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func Test_getAccessibleVolumeMounts(t *testing.T) {
	existingPath := "./existing-folder"
	missingPath := "./missing-folder"
	buildInfo := &build.Info{
		VolumesToInclude: []build.VolumeMounts{
			{LocalPath: existingPath, RemotePath: "/data/logs"},
			{LocalPath: missingPath, RemotePath: "/data/logs"},
		},
	}
	err := os.Mkdir(existingPath, 0750)
	if err != nil {
		t.Fatal(err)
	}
	volumes := getAccessibleVolumeMounts(buildInfo)
	err = os.Remove(existingPath)
	assert.NoError(t, err)
	assert.Len(t, volumes, 1)
}

func createDockerfile(t *testing.T) (string, error) {
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	err := os.WriteFile(dockerfilePath, []byte("Hello"), 0600)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func TestBuildWithDependsOn(t *testing.T) {
	ctx := context.Background()

	firstImage := "okteto/a:test"
	secondImage := "okteto/b:test"
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}

	bc := NewFakeBuilder(builder, registry, fakeConfig, &fakeAnalyticsTracker{})
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"a": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Image:      firstImage,
			},
			"b": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Image:      secondImage,
				DependsOn:  []string{"a"},
			},
		},
	}
	err = bc.Build(ctx, &types.BuildOptions{
		Manifest: manifest,
	})

	// error from the build
	assert.NoError(t, err)

	// check that images are on the registry
	_, err = registry.GetImageTagWithDigest(firstImage)
	assert.NoError(t, err)

	_, err = registry.GetImageTagWithDigest(secondImage)
	assert.NoError(t, err)

	expectedKeys := map[string]bool{
		"OKTETO_BUILD_A_IMAGE":      false,
		"OKTETO_BUILD_A_REGISTRY":   false,
		"OKTETO_BUILD_A_REPOSITORY": false,
		"OKTETO_BUILD_A_TAG":        false,
		"OKTETO_BUILD_A_SHA":        false,
	}
	for _, arg := range registry.getFakeImage(secondImage).Args {
		parts := strings.SplitN(arg, "=", 2)
		if _, ok := expectedKeys[parts[0]]; ok {
			expectedKeys[parts[0]] = true
		}
	}
	for k, v := range expectedKeys {
		if !v {
			t.Fatalf("expected to inject '%s' on image '%s' but is not injected", k, secondImage)
		}
	}

}

func Test_areAllServicesBuilt(t *testing.T) {
	tests := []struct {
		name     string
		control  map[string]bool
		input    []string
		expected bool
	}{
		{
			name:     "all built",
			expected: true,
			input:    []string{"one", "two", "three"},
			control: map[string]bool{
				"one":   true,
				"two":   true,
				"three": true,
			},
		},
		{
			name:     "none built",
			expected: false,
			input:    []string{"one", "two", "three"},
			control:  map[string]bool{},
		},
		{
			name:     "some built",
			expected: false,
			input:    []string{"one", "two", "three"},
			control: map[string]bool{
				"one": true,
				"two": true,
			},
		},
		{
			name:     "nil control",
			expected: false,
			input:    []string{"one", "two", "three"},
		},
		{
			name:     "nil input",
			expected: true,
			control: map[string]bool{
				"one": true,
				"two": true,
			},
		},
		{
			name:     "empty input",
			expected: true,
			input:    []string{},
			control: map[string]bool{
				"one": true,
				"two": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := areAllServicesBuilt(tt.input, tt.control)
			require.Equal(t, tt.expected, got)
		})

	}
}

func Test_skipServiceBuild(t *testing.T) {
	tests := []struct {
		name     string
		control  map[string]bool
		input    string
		expected bool
	}{
		{
			name:     "is built",
			expected: true,
			input:    "one",
			control: map[string]bool{
				"one":   true,
				"two":   true,
				"three": true,
			},
		},
		{
			name:     "not built",
			expected: false,
			input:    "one",
			control:  map[string]bool{},
		},
		{
			name:     "nil control",
			expected: false,
			input:    "one",
		},
		{
			name:     "empty input",
			expected: false,
			control: map[string]bool{
				"one": true,
				"two": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skipServiceBuild(tt.input, tt.control)
			require.Equal(t, tt.expected, got)
		})

	}
}
