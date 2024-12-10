//go:build integration
// +build integration

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

package deploy

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

var (
	deployRemoteWithBuildCommandManifestContent = `
deploy:
  remote: true
  commands:
    - okteto build -t okteto.dev/testing-image:test -f app/Dockerfile --no-cache`
)

// TestDeployRemoteWithBuildCommand tests the following scenario:
// - Deploying a okteto manifest in remote with a build command
func TestDeployRemoteWithBuildCommand(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	require.NoError(t, createOktetoManifest(dir, deployRemoteWithBuildCommandManifestContent))
	require.NoError(t, createAppDockerfile(dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image has been built
	require.NotEmpty(t, getImageWithSHA(fmt.Sprintf("%s/%s/testing-image:test", okteto.GetContext().Registry, testNamespace)))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroyRemote(oktetoPath, destroyOptions))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployRemoteWithBuildCommand tests the following scenario:
// - Deploying a okteto manifest with a k8s and there is a dockerignore file containing the k8s.yml
func TestDeployRemoteK8sWithDockerignore(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	require.NoError(t, createPipelineManifest(dir))
	require.NoError(t, createK8sManifest(dir))
	require.NoError(t, createDockerignore(dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		IsRemote:   true,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroyRemote(oktetoPath, destroyOptions))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

func createDockerignore(dir string) error {
	manifestPath := filepath.Join(dir, ".dockerignore")
	manifestContent := []byte("k8s.yml")
	if err := os.WriteFile(manifestPath, manifestContent, 0600); err != nil {
		return err
	}
	return nil
}
