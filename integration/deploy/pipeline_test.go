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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	pipelineDeployPipelineManifestName = "okteto-pipeline-inside.yml"
	pipelineDeployPipelineManifest     = `
deploy:
  - %s deploy -f okteto-pipeline.yml --name=test --namespace=%s
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
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createPipelineManifest(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("DeployPipeline", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		Token:      token,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))

}

// TestDeployPipelineManifestInsidePipeline tests the following scenario:
// - Deploying a pipeline manifest locally that runs another pipeline manifest
// - The endpoints generated are accessible
func TestDeployPipelineManifestInsidePipeline(t *testing.T) {
	integration.SkipIfWindows(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)
	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace("Deploy", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	require.NoError(t, createPipelineInsidePipelineManifest(dir, oktetoPath, testNamespace))
	require.NoError(t, createK8sManifest(dir))

	deployOptions := &commands.DeployOptions{
		Workdir:      dir,
		ManifestPath: pipelineDeployPipelineManifestName,
		Namespace:    testNamespace,
		OktetoHome:   dir,
		Token:        token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	// Check that the e2etest service is still running after destroying the outer pipeline
	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.NoError(t, err)

	destroyOptions = &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Name:       "test",
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	// Check that the e2etest service is not running
	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

func createPipelineInsidePipelineManifest(dir, oktetoPath, namespace string) error {
	dockerfilePath := filepath.Join(dir, pipelineManifestName)
	dockerfileContent := []byte(pipelineManifest)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	dockerfilePath = filepath.Join(dir, pipelineDeployPipelineManifestName)
	dockerfileContent = []byte(fmt.Sprintf(pipelineDeployPipelineManifest, oktetoPath, namespace))
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createPipelineManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, pipelineManifestName)
	dockerfileContent := []byte(pipelineManifest)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createK8sManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, k8sManifestName)
	dockerfileContent := []byte(k8sManifestTemplate)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}
