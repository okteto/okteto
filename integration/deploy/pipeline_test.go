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
	pipelineDeployPipelineManifestName = "okteto-pipeline.yml"
	pipelineDeployPipelineManifest     = `
deploy:
  - %s deploy -f okteto-pipeline.yml
  - kubectl get pods`

	pipelineManifestName = "okteto-pipeline.yml"
	pipelineManifest     = `
deploy:
  - kubectl apply -f k8s.yml
`
)

// TestDeployPipelineManifest tests the following scenario:
// - Deploying a pipeline manifest locally
// - The endpoints generated are accessible
func TestDeployPipelineManifest(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createPipelineManifest(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestDeploy", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	deployOptions := &commands.DeployOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

}

// TestDeployPipelineManifest tests the following scenario:
// - Deploying a pipeline manifest locally that runs another pipeline manifest
// - The endpoints generated are accessible
func TestDeployPipelineManifestInsidePipeline(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createPipelineInsidePipelineManifest(dir, oktetoPath))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestDeploy", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	deployOptions := &commands.DeployOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}

func createPipelineInsidePipelineManifest(dir, oktetoPath string) error {
	dockerfilePath := filepath.Join(dir, pipelineManifestName)
	dockerfileContent := []byte(pipelineManifest)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	dockerfilePath = filepath.Join(dir, pipelineDeployPipelineManifestName)
	dockerfileContent = []byte(fmt.Sprintf(pipelineDeployPipelineManifest, oktetoPath))
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	return nil
}

func createPipelineManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, pipelineManifestName)
	dockerfileContent := []byte(pipelineManifest)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	return nil
}

func createK8sManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, k8sManifestName)
	dockerfileContent := []byte(k8sManifestTemplate)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	return nil
}
