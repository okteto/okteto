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

package v1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeRegistry struct {
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

func (fr fakeRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fr.registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}

func (fr fakeRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fr.registry[opts.Tag] = fakeImage{Args: opts.BuildArgs}
	return nil
}

func (fr fakeRegistry) HasGlobalPushAccess() (bool, error) { return false, nil }

func TestBuildWithErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry, fmt.Errorf("failed to build error"))
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
		IoCtrl:   io.NewIOController(),
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	tag := "okteto.dev/test"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}
	err = bc.Build(ctx, options)

	// error from the build
	assert.Error(t, err)
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(options.Tag)
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
}

func TestBuildWithErrorFromImageExpansion(t *testing.T) {
	ctx := context.Background()

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
		IoCtrl:   io.NewIOController(),
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	t.Setenv("TEST_VAR", "unit-test")
	// The missing closing brace breaks the var expansion
	tag := "okteto.dev/test:${TEST_VAR"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}
	err = bc.Build(ctx, options)
	// error from the build
	assert.ErrorAs(t, err, &env.VarExpansionErr{})
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(options.Tag)
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
}

func TestBuildWithNoErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
		IoCtrl:   io.NewIOController(),
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	t.Setenv("TEST_VAR", "unit-test")
	tag := "okteto.dev/test:${TEST_VAR}"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}
	err = bc.Build(ctx, options)
	// no error from the build
	assert.NoError(t, err)
	// the image is at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(options.Tag)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestBuildWithNoErrorFromDockerfileAndNoTag(t *testing.T) {
	ctx := context.Background()

	registry := newFakeRegistry()
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
		IoCtrl:   io.NewIOController(),
	}
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	options := &types.BuildOptions{
		CommandArgs: []string{dir},
	}
	err = bc.Build(ctx, options)
	// no error from the build
	assert.NoError(t, err)
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest("")
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
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
