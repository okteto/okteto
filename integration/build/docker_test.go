//go:build integration
// +build integration

// Copyright 2026 The Okteto Authors
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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

// These tests need an Okteto instance with the Docker daemon of BuildKit enabled (buildkit.dind.enabled)

const (
	// dockerfileContentDocker runs a container in a RUN step. It doesn't declare DOCKER_HOST: the CLI does it
	dockerfileContentDocker = `FROM docker:29-cli
RUN docker run --rm alpine echo hello`

	manifestContentDocker = `
build:
  app:
    context: .
    docker: true
`
	testcontainersTestdata = "testdata/testcontainers"
)

// TestBuildWithDocker tests the following scenario:
// - building with `docker: true` a Dockerfile that runs a container and doesn't declare DOCKER_HOST
func TestBuildWithDocker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, dockerfileName), []byte(dockerfileContentDocker), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, manifestName), []byte(manifestContentDocker), 0600))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	expectedImage := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(expectedImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, manifestName),
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedImage))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestBuildWithDockerTestcontainers tests the following scenario:
// - building with `docker: true` a Maven project whose tests use Testcontainers, running them inside the build
func TestBuildWithDockerTestcontainers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(testcontainersTestdata)))

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	expectedImage := fmt.Sprintf("%s/%s/testcontainers-example-app:okteto", okteto.GetContext().Registry, testNamespace)
	require.False(t, isImageBuilt(expectedImage))

	options := &commands.BuildOptions{
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, manifestName),
		Namespace:    testNamespace,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
	require.True(t, isImageBuilt(expectedImage))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
