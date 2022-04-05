// Copyright 2022 The Okteto Authors
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

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateOptions(t *testing.T) {
	var tests = []struct {
		name         string
		buildSection model.ManifestBuild
		svcsToBuild  []string
		options      types.BuildOptions
		expectedErr  bool
	}{
		{
			name:         "no services to build",
			buildSection: model.ManifestBuild{},
			svcsToBuild:  []string{},
			options:      types.BuildOptions{},
			expectedErr:  true,
		},
		{
			name:         "svc not defined on manifest build section",
			buildSection: model.ManifestBuild{},
			svcsToBuild:  []string{"test"},
			options:      types.BuildOptions{},
			expectedErr:  true,
		},
		{
			name: "several services but with flag",
			buildSection: model.ManifestBuild{
				"test":   &model.BuildInfo{},
				"test-2": &model.BuildInfo{},
			},
			svcsToBuild: []string{"test", "test-2"},
			options: types.BuildOptions{
				Tag: "test",
			},
			expectedErr: true,
		},
		{
			name: "only one service without flags",
			buildSection: model.ManifestBuild{
				"test": &model.BuildInfo{},
			},
			svcsToBuild: []string{"test"},
			options:     types.BuildOptions{},
			expectedErr: false,
		},
		{
			name: "only one service with flags",
			buildSection: model.ManifestBuild{
				"test": &model.BuildInfo{},
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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	manifest := &model.Manifest{
		Name: "test",
		Build: model.ManifestBuild{
			"test": &model.BuildInfo{
				Image: "nginx",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  dir,
						RemotePath: "test",
					},
				},
			},
		},
	}
	image, err := bc.buildService(ctx, manifest, "test", &types.BuildOptions{})

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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = createDockerfile()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	manifest := &model.Manifest{
		Name: "test",
		Build: model.ManifestBuild{
			"test": &model.BuildInfo{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  dir,
						RemotePath: "test",
					},
				},
			},
		},
	}
	image, err := bc.buildService(ctx, manifest, "test", &types.BuildOptions{})

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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}

	dir, err := createDockerfile()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	manifest := &model.Manifest{
		Name: "test",
		Build: model.ManifestBuild{
			"test": &model.BuildInfo{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
			},
		},
	}
	image, err := bc.buildService(ctx, manifest, "test", &types.BuildOptions{})

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
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}

	dir, err := createDockerfile()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	manifest := &model.Manifest{
		Name: "test",
		Build: model.ManifestBuild{
			"test": &model.BuildInfo{
				Context:    dir,
				Dockerfile: filepath.Join(dir, "Dockerfile"),
				Image:      "okteto/test",
			},
		},
	}
	image, err := bc.buildService(ctx, manifest, "test", &types.BuildOptions{})

	// error from the build
	assert.NoError(t, err)
	// assert that the name of the image is the dev one
	assert.Equal(t, "okteto/test", image)
	// the image is at the fake registry
	image, err = bc.Registry.GetImageTagWithDigest(image)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func Test_getAccessibleVolumeMounts(t *testing.T) {
	existingPath := "./existing-folder"
	missingPath := "./missing-folder"
	buildInfo := &model.BuildInfo{
		VolumesToInclude: []model.StackVolume{
			{LocalPath: existingPath, RemotePath: "/data/logs"},
			{LocalPath: missingPath, RemotePath: "/data/logs"},
		},
	}
	err := os.Mkdir(existingPath, 0755)
	if err != nil {
		t.Fatal(err)
	}
	volumes := getAccessibleVolumeMounts(buildInfo)
	err = os.Remove(existingPath)
	assert.NoError(t, err)
	assert.Len(t, volumes, 1)
}
