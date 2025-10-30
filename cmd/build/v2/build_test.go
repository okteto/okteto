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
	"github.com/okteto/okteto/cmd/build/basic"
	"github.com/okteto/okteto/cmd/build/v2/environment"
	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
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
}

type fakeWorkingDirGetter struct{}

func (f fakeWorkingDirGetter) Get() (string, error) {
	return "", nil
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
func (fr fakeRegistry) Clone(from, to string) (string, error) {
	return from, nil
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

func (fr fakeRegistry) GetRegistryAndRepo(image string) (string, string)    { return "", "" }
func (fr fakeRegistry) GetRepoNameAndTag(repo string) (string, string)      { return "", "" }
func (fr fakeRegistry) GetDevImageFromGlobal(imageWithDigest string) string { return "" }

type fakeImageConfig struct{}

func (fic *fakeImageConfig) IsOktetoCluster() bool {
	return true
}

func (fic *fakeImageConfig) GetGlobalNamespace() string {
	return "okteto-global"
}

func (fic *fakeImageConfig) GetNamespace() string {
	return "test"
}

func (fic *fakeImageConfig) GetRegistryURL() string {
	return "my-registry"
}

func NewFakeBuilder(builder buildCmd.OktetoBuilderInterface, registryIface oktetoRegistryInterface, cfg oktetoBuilderConfigInterface) *OktetoBuilder {
	ioCtrl := io.NewIOController()
	// Smart builds are enabled by default, and now sequential strategy is used as fallback
	// when parallel is not implemented, so CheckStrategy should never be nil
	config := smartbuild.NewConfig()
	imageCtrl := registry.NewImageCtrl(&fakeImageConfig{})
	smartBuildCtrl := smartbuild.NewSmartBuildCtrl(
		fakeConfigRepo{},
		registryIface,
		afero.NewMemMapFs(),
		ioCtrl,
		fakeWorkingDirGetter{},
		config,
		newImageTagger(cfg, nil),
		imageCtrl,
		environment.NewServiceEnvVarsHandler(ioCtrl, registryIface),
		"test-namespace",
		"test-registry.com",
	)
	return &OktetoBuilder{
		Registry: registryIface,
		Builder: basic.Builder{
			BuildRunner: builder,
			IoCtrl:      ioCtrl,
		},
		Config:         cfg,
		ioCtrl:         ioCtrl,
		smartBuildCtrl: smartBuildCtrl,
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
		serviceEnvVarsHandler: environment.NewServiceEnvVarsHandler(ioCtrl, registryIface),
	}
}

func TestMain(m *testing.M) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {},
		},
		CurrentContext: "test",
	}
	os.Exit(m.Run())
}

func TestValidateOptions(t *testing.T) {
	var tests = []struct {
		buildSection build.ManifestBuild
		name         string
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

func TestTwoStepsBuild(t *testing.T) {
	ctx := context.Background()

	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(builder, registry, fakeConfig)
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
	infos := buildTypes.NewBuildInfos(manifest.Name, "test", "", []string{"test"})
	image, err := bc.buildServiceImages(ctx, manifest, infos[0], &types.BuildOptions{})

	require.NoError(t, err)
	require.Equal(t, "okteto.dev/test-test:okteto", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
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
	bc := NewFakeBuilder(builder, registry, fakeConfig)
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"test": &build.Info{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
			},
		},
	}
	infos := buildTypes.NewBuildInfos(manifest.Name, "test", "", []string{"test"})
	image, err := bc.buildServiceImages(ctx, manifest, infos[0], &types.BuildOptions{})

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
	bc := NewFakeBuilder(builder, registry, fakeConfig)
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
	infos := buildTypes.NewBuildInfos(manifest.Name, "test", "", []string{"test"})
	image, err := bc.buildServiceImages(ctx, manifest, infos[0], &types.BuildOptions{})

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
	bc := NewFakeBuilder(builder, registry, fakeConfig)
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
	infos := buildTypes.NewBuildInfos(manifest.Name, "test", "", []string{"test"})
	image, err := bc.buildServiceImages(ctx, manifest, infos[0], &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto.dev/test-test:okteto", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
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

	bc := NewFakeBuilder(builder, registry, fakeConfig)
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
