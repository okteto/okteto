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
	oktetoManifestContentWithCache = `build:
  app:
    context: app
    export_cache: okteto.dev/app:dev
    cache_from: okteto.dev/app:dev
    image: okteto.dev/app:dev
deploy:
  - kubectl apply -f k8s.yml
`
	oktetoManifestWithDeployRemote = `build:
  app:
    context: app
    image: okteto.dev/app:dev
deploy:
  image: okteto/installer:1.8.9
  commands:
  - name: deploy nginx
    command: kubectl create deployment my-dep --image=busybox`

	appDockerfileWithCache = `FROM python:alpine
EXPOSE 2931
RUN --mount=type=cache,target=/root/.cache echo hola`
	k8sManifestTemplateWithCache = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2etest
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2etest
  template:
    metadata:
      labels:
        app: e2etest
    spec:
      terminationGracePeriodSeconds: 1
      containers:
      - name: test
        image: %s
        ports:
        - containerPort: 8080
        workingDir: /usr/src/app
        env:
          - name: VAR
            value: value1
        command:
            - sh
            - -c
            - "echo -n $VAR > var.html && python -m http.server 8080"
---
apiVersion: v1
kind: Service
metadata:
  name: e2etest
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: e2etest
    port: 8080
  selector:
    app: e2etest
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

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
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
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
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
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
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

// TestDeployOktetoManifestExportCache tests the following scenario:
// - Deploying a okteto manifest locally with a build that has a export cache
func TestDeployOktetoManifestExportCache(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace("TestDeployExportCache", user)
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

	require.NoError(t, createOktetoManifestWithCache(dir))
	require.NoError(t, createAppDockerfileWithCache(dir))
	require.NoError(t, createK8sManifestWithCache(dir, testNamespace))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image has been built
	require.NotEmpty(t, getImageWithSHA(fmt.Sprintf("%s/%s/app:dev", okteto.GetContext().Registry, testNamespace)))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

// TestDeployRemoteOktetoManifest tests the following scenario:
// - Deploying a okteto manifest in remote with a build locally
func TestDeployRemoteOktetoManifest(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace("TestDeployRemote", user)
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

	require.NoError(t, createOktetoManifestWithDeployRemote(dir))
	require.NoError(t, createAppDockerfileWithCache(dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image has been built
	require.NotEmpty(t, getImageWithSHA(fmt.Sprintf("%s/%s/app:dev", okteto.GetContext().Registry, testNamespace)))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroyRemote(oktetoPath, destroyOptions))

	_, err = integration.GetDeployment(context.Background(), testNamespace, "my-dep", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

// TestDeployRemoteOktetoManifest tests the following scenario:
// - Deploying a okteto manifest in remote with a build locally
func TestDeployRemoteOktetoManifestFromParentFolder(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	parentFolder := filepath.Join(dir, "test-parent")

	testNamespace := integration.GetTestNamespace("TestDeployRemoteParent", user)
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

	require.NoError(t, createOktetoManifestWithDeployRemote(dir))
	require.NoError(t, createAppDockerfileWithCache(dir))
	require.NoError(t, os.Mkdir(parentFolder, 0700))

	deployOptions := &commands.DeployOptions{
		Workdir:      parentFolder,
		Namespace:    testNamespace,
		OktetoHome:   dir,
		Token:        token,
		ManifestPath: filepath.Clean("../okteto.yml"),
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image has been built
	require.NotEmpty(t, getImageWithSHA(fmt.Sprintf("%s/%s/app:dev", okteto.GetContext().Registry, testNamespace)))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroyRemote(oktetoPath, destroyOptions))

	_, err = integration.GetDeployment(context.Background(), testNamespace, "my-dep", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

func isImageBuilt(image string) bool {
	reg := registry.NewOktetoRegistry(okteto.Config{})
	if _, err := reg.GetImageTagWithDigest(image); err == nil {
		return true
	}
	return false
}

func createOktetoManifestWithDeployRemote(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestWithDeployRemote)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createOktetoManifestWithCache(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestContentWithCache)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createOktetoManifest(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
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

func createK8sManifestWithCache(dir, ns string) error {
	dockerfilePath := filepath.Join(dir, k8sManifestName)
	appImageDev := fmt.Sprintf("%s/%s/app:dev", okteto.GetContext().Registry, ns)

	dockerfileContent := []byte(fmt.Sprintf(k8sManifestTemplateWithCache, appImageDev))
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createAppDockerfileWithCache(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "app"), 0700); err != nil {
		return err
	}

	appDockerfilePath := filepath.Join(dir, "app", "Dockerfile")
	appDockerfileContent := []byte(appDockerfileWithCache)
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}
