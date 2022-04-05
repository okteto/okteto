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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildWithErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry, fmt.Errorf("failed to build error"))
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	dir, err := createDockerfile()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	tag := "okteto.dev/test"
	options := &types.BuildOptions{
		Args: []string{dir},
		Tag:  tag,
	}
	err = bc.BuildV1(ctx, options)

	// error from the build
	assert.Error(t, err)
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(tag)
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
}
func TestBuildWithNoErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	dir, err := createDockerfile()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	tag := "okteto.dev/test"
	options := &types.BuildOptions{
		Args: []string{dir},
		Tag:  tag,
	}
	err = bc.BuildV1(ctx, options)
	// no error from the build
	assert.NoError(t, err)
	// the image is at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest(tag)
	assert.NoError(t, err)
	assert.NotEmpty(t, image)
}

func TestBuildWithNoErrorFromDockerfileAndNoTag(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}

	registry := test.NewFakeOktetoRegistry(nil)
	builder := test.NewFakeOktetoBuilder(registry)
	bc := &Command{
		Builder:  builder,
		Registry: registry,
	}
	dir, err := createDockerfile()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	options := &types.BuildOptions{
		Args: []string{dir},
	}
	err = bc.BuildV1(ctx, options)
	// no error from the build
	assert.NoError(t, err)
	// the image is not at the fake registry
	image, err := bc.Registry.GetImageTagWithDigest("")
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
	assert.Empty(t, image)
}

func createDockerfile() (string, error) {
	dir, err := os.MkdirTemp("", "build")
	if err != nil {
		return "", err
	}
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	err = os.WriteFile(dockerfilePath, []byte("Hello"), 0755)
	if err != nil {
		return "", err
	}
	return dir, nil
}
