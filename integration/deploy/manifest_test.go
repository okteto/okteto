//go:build integration
// +build integration

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

package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

var (
	oktetoManifestName    = "okteto.yml"
	oktetoManifestContent = `build:
  app:
    context: app
deploy:
  - kubectl apply -f k8s.yml
`
)

// TestDeployOktetoManifest tests the following scenario:
// - Deploying a okteto manifest locally
// - The endpoints generated are accessible
func TestDeployOktetoManifest(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createOktetoManifest(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestDeploy", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	deployOptions := &commands.DeployOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that endpoint works
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, getContentFromURL(autowakeURL, timeout))

	// Test that image has been built
	appImageDev := fmt.Sprintf("okteto.dev/%s-app:okteto", filepath.Base(dir))
	require.NotEmpty(t, getImageWithSHA(appImageDev))

	destroyOptions := &commands.DestroyOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

}

func createOktetoManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	return nil
}
