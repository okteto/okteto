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
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeBuildRunner struct {
	mock.Mock
}

func (f *fakeBuildRunner) Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller, varManager *vars.Manager) error {
	args := f.Called(ctx, buildOptions, ioCtrl, varManager)
	return args.Error(0)
}

type varManagerLogger struct{}

func (varManagerLogger) Yellow(_ string, _ ...interface{}) {}
func (varManagerLogger) AddMaskedWord(_ string)            {}

func TestBuildWithErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()

	varManager := vars.NewVarsManager(&varManagerLogger{})
	buildRunner := &fakeBuildRunner{}
	bc := NewBuilder(buildRunner, io.NewIOController(), varManager)
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	tag := "okteto.dev/test"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}

	expectedOptions := &types.BuildOptions{
		Path:        dir,
		File:        filepath.Join(dir, "Dockerfile"),
		Tag:         tag,
		CommandArgs: []string{dir},
	}
	buildRunner.On("Run", mock.Anything, expectedOptions, mock.Anything, mock.Anything).Return(assert.AnError)

	err = bc.Build(ctx, options)

	// error from the build
	assert.Error(t, err)

	buildRunner.AssertExpectations(t)
}

func TestBuildWithErrorFromImageExpansion(t *testing.T) {
	ctx := context.Background()

	varManager := vars.NewVarsManager(&varManagerLogger{})
	varManager.AddLocalVar("TEST_VAR", "unit-test")

	buildRunner := &fakeBuildRunner{}
	bc := NewBuilder(buildRunner, io.NewIOController(), varManager)
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	// The missing closing brace breaks the var expansion
	tag := "okteto.dev/test:${TEST_VAR"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}
	err = bc.Build(ctx, options)

	// error from the build
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "closing brace expected")

	buildRunner.AssertNotCalled(t, "Run", mock.Anything, mock.Anything, mock.Anything)
}

func TestBuildWithNoErrorFromDockerfile(t *testing.T) {
	ctx := context.Background()

	varManager := vars.NewVarsManager(&varManagerLogger{})
	varManager.AddLocalVar("TEST_VAR", "unit-test")

	buildRunner := &fakeBuildRunner{}
	bc := NewBuilder(buildRunner, io.NewIOController(), varManager)
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	tag := "okteto.dev/test:${TEST_VAR}"
	options := &types.BuildOptions{
		CommandArgs: []string{dir},
		Tag:         tag,
	}

	expectedOptions := &types.BuildOptions{
		Path:        dir,
		File:        filepath.Join(dir, "Dockerfile"),
		Tag:         "okteto.dev/test:unit-test",
		CommandArgs: []string{dir},
	}
	buildRunner.On("Run", mock.Anything, expectedOptions, mock.Anything, mock.Anything).Return(nil)

	err = bc.Build(ctx, options)
	// no error from the build
	assert.NoError(t, err)

	buildRunner.AssertExpectations(t)
}

func TestBuildWithNoErrorFromDockerfileAndNoTag(t *testing.T) {
	ctx := context.Background()

	varManager := vars.NewVarsManager(&varManagerLogger{})
	buildRunner := &fakeBuildRunner{}
	bc := NewBuilder(buildRunner, io.NewIOController(), varManager)
	dir, err := createDockerfile(t)
	assert.NoError(t, err)

	options := &types.BuildOptions{
		CommandArgs: []string{dir},
	}

	expectedOptions := &types.BuildOptions{
		Path:        dir,
		File:        filepath.Join(dir, "Dockerfile"),
		CommandArgs: []string{dir},
	}
	buildRunner.On("Run", mock.Anything, expectedOptions, mock.Anything, mock.Anything).Return(nil)

	err = bc.Build(ctx, options)
	// no error from the build
	assert.NoError(t, err)

	buildRunner.AssertExpectations(t)
}

func TestIsV1(t *testing.T) {
	bc := &OktetoBuilder{}
	assert.True(t, bc.IsV1())
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
