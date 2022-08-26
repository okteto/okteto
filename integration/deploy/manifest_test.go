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
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	oktetoManifestName    = "okteto.yml"
	oktetoManifestContent = `build:
  app:
    context: app
deploy:
  - kubectl apply -f k8s.yml
`
	oktetoManifestWithDestroyContent = `build:
app:
  context: app
deploy:
- okteto destroy
- kubectl apply -f k8s.yml
`
)

// TestDeployOktetoManifest tests the following scenario:
// - Deploying a okteto manifest locally
// - The endpoints generated are accessible
func TestDeployOktetoManifest(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createOktetoManifest(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestDeployManifestV2", user)
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

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that endpoint works
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	// Test that image has been built

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	require.NotEmpty(t, getImageWithSHA(appImageDev))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

// TestDeployOktetoManifest tests the following scenario:
// - Deploying a okteto manifest locally
// - The endpoints generated are accessible
// - Images are only build if
func TestRedeployOktetoManifestForImages(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createOktetoManifest(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestReDeploy", user)
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

	// Test that image is not built before running okteto deploy
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(appImageDev))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image is built after running okteto deploy
	require.True(t, isImageBuilt(appImageDev))

	// Test that endpoint works
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	deployOptions.LogLevel = "debug"
	// Test redeploy is not building any image
	output, err := commands.RunOktetoDeployAndGetOutput(oktetoPath, deployOptions)
	require.NoError(t, err)

	err = expectImageFoundSkippingBuild(output)
	require.NoError(t, err, err)

	// Test redeploy with build flag builds the image
	deployOptions.Build = true
	output, err = commands.RunOktetoDeployAndGetOutput(oktetoPath, deployOptions)
	require.NoError(t, err)

	require.NoError(t, expectForceBuild(output))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

// TestDeployOktetoManifestWithDestroy tests the following scenario:
// - Deploying a okteto manifest locally
// - The endpoints generated are accessible
// - Redeploy with okteto deploy
// - Checks that configmap is still there
func TestDeployOktetoManifestWithDestroy(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createOktetoManifest(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestDeployDestroy", user)
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

	// Test that image is not built before running okteto deploy
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.Context().Registry, testNamespace, filepath.Base(dir))
	require.False(t, isImageBuilt(appImageDev))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image is built after running okteto deploy
	require.True(t, isImageBuilt(appImageDev))

	// Test that endpoint works
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	deployOptions.LogLevel = "debug"
	// Test redeploy is not building any image
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	_, err = integration.GetConfigmap(context.Background(), testNamespace, fmt.Sprintf("okteto-git-%s", filepath.Base(dir)), c)
	require.NoError(t, err)

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

func isImageBuilt(image string) bool {
	reg := registry.NewOktetoRegistry()
	if _, err := reg.GetImageTagWithDigest(image); err == nil {
		return true
	}
	return false
}

func createOktetoManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return err
	}
	return nil
}

func expectImageFoundSkippingBuild(output string) error {
	if ok := strings.Contains(output, "Skipping build for image for service"); !ok {
		log.Print(output)
		return errors.New("expected image found, skipping build")
	}
	return nil
}

func expectForceBuild(output string) error {
	if ok := strings.Contains(output, "force build from manifest definition"); !ok {
		return errors.New("expected force build from manifest definition")
	}
	return nil
}
