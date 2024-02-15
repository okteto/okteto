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

package up

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

const (
	autocreateManifest = `
name: autocreate
image: python:alpine
command:
  - sh
  - -c
  - "echo -n $VAR > var.html && python -m http.server 8080"
environment:
  - VAR=value1
workdir: /usr/src/app
sync:
- .:/usr/src/app
forward:
- 8080:8080
autocreate: true
`
	autocreateManifestV2 = `
dev:
  autocreate:
    image: python:alpine
    command:
    - sh
    - -c
    - "echo -n $VAR > var.html && python -m http.server 8080"
    environment:
    - VAR=value1
    workdir: /usr/src/app
    sync:
    - .:/usr/src/app
    forward:
    - 8081:8080
    autocreate: true
`

	autocreateManifestV2WithBuild = `
build:
  autocreate:
    context: .
dev:
  autocreate:
    image: ${OKTETO_BUILD_AUTOCREATE_IMAGE}
    command:
    - sh
    - -c
    - "echo -n $VAR > var.html && python -m http.server 8080"
    environment:
    - VAR=value1
    workdir: /usr/src/app
    sync:
    - .:/usr/src/app
    forward:
    - 8012:8080
    autocreate: true
`
)

func TestUpAutocreate(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("UpAutocreateV1", user)
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

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), autocreateManifest))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	upOptions := &commands.UpOptions{
		Name:         "autocreate",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		Name:       model.DevCloneName("autocreate"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	varLocalEndpoint := "http://localhost:8080/var.html"
	indexLocalEndpoint := "http://localhost:8080/index.html"
	indexRemoteEndpoint := fmt.Sprintf("https://autocreate-%s.%s/index.html", testNamespace, appsSubdomain)

	// Test that environment variable is injected correctly
	require.Equal(t, integration.GetContentFromURL(varLocalEndpoint, timeout), "value1")

	// Test that the same content is on the remote and on local endpoint
	require.NotEmpty(t, integration.GetContentFromURL(indexLocalEndpoint, timeout))
	require.Equal(t, integration.GetContentFromURL(indexLocalEndpoint, timeout), testNamespace)
	require.Equal(t, integration.GetContentFromURL(indexLocalEndpoint, timeout), integration.GetContentFromURL(indexRemoteEndpoint, timeout))

	// Test that making a change gets reflected on remote
	localupdatedContent := fmt.Sprintf("%s-updated-content", testNamespace)
	require.NoError(t, writeFile(indexPath, localupdatedContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localupdatedContent, timeout, upResult.ErrorChan))

	// Test that stignore has been created
	require.NoError(t, checkStignoreIsOnRemote(testNamespace, filepath.Join(dir, "okteto.yml"), oktetoPath, dir))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=autocreate", c))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace:    testNamespace,
		ManifestPath: upOptions.ManifestPath,
		Workdir:      dir,
		Token:        token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))
}

func TestUpAutocreateV2(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("UpAutocreateV2", user)
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

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), autocreateManifestV2))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	upOptions := &commands.UpOptions{
		Name:         "autocreate",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		Name:       model.DevCloneName("autocreate"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	varLocalEndpoint := "http://localhost:8081/var.html"
	indexLocalEndpoint := "http://localhost:8081/index.html"
	indexRemoteEndpoint := fmt.Sprintf("https://autocreate-%s.%s/index.html", testNamespace, appsSubdomain)

	// Test that environment variable is injected correctly
	require.NoError(t, waitUntilUpdatedContent(varLocalEndpoint, "value1", timeout, upResult.ErrorChan))

	// Test that the same content is on the remote and on local endpoint
	require.NotEmpty(t, integration.GetContentFromURL(indexLocalEndpoint, timeout))
	require.Equal(t, integration.GetContentFromURL(indexLocalEndpoint, timeout), testNamespace)
	require.Equal(t, integration.GetContentFromURL(indexLocalEndpoint, timeout), integration.GetContentFromURL(indexRemoteEndpoint, timeout))

	// Test that making a change gets reflected on remote
	localupdatedContent := fmt.Sprintf("%s-updated-content", testNamespace)
	require.NoError(t, writeFile(indexPath, localupdatedContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localupdatedContent, timeout, upResult.ErrorChan))

	// Test that stignore has been created
	require.NoError(t, checkStignoreIsOnRemote(testNamespace, filepath.Join(dir, "okteto.yml"), oktetoPath, dir))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=autocreate", c))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace:    testNamespace,
		ManifestPath: upOptions.ManifestPath,
		Workdir:      dir,
		Token:        token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))
}

func TestUpAutocreateV2WithBuild(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("UpAutocreateV2Build", user)
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

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), autocreateManifestV2WithBuild))
	require.NoError(t, writeFile(filepath.Join(dir, "Dockerfile"), "FROM python:alpine"))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))
	require.NoError(t, writeFile(filepath.Join(dir, ".dockerignore"), stignoreContent))
	upOptions := &commands.UpOptions{
		Name:         "autocreate",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		Name:       model.DevCloneName("autocreate"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	varLocalEndpoint := "http://localhost:8012/var.html"
	indexLocalEndpoint := "http://localhost:8012/index.html"
	indexRemoteEndpoint := fmt.Sprintf("https://autocreate-%s.%s/index.html", testNamespace, appsSubdomain)

	// Test that environment variable is injected correctly
	require.NoError(t, waitUntilUpdatedContent(varLocalEndpoint, "value1", timeout, upResult.ErrorChan))

	// Test that the same content is on the remote and on local endpoint
	require.NotEmpty(t, integration.GetContentFromURL(indexLocalEndpoint, timeout))
	require.Equal(t, integration.GetContentFromURL(indexLocalEndpoint, timeout), testNamespace)
	require.Equal(t, integration.GetContentFromURL(indexLocalEndpoint, timeout), integration.GetContentFromURL(indexRemoteEndpoint, timeout))

	// Test that making a change gets reflected on remote
	localupdatedContent := fmt.Sprintf("%s-updated-content", testNamespace)
	require.NoError(t, writeFile(indexPath, localupdatedContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localupdatedContent, timeout, upResult.ErrorChan))

	// Test that stignore has been created
	require.NoError(t, checkStignoreIsOnRemote(testNamespace, filepath.Join(dir, "okteto.yml"), oktetoPath, dir))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=autocreate", c))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace:    testNamespace,
		ManifestPath: upOptions.ManifestPath,
		Workdir:      dir,
		Token:        token,
		OktetoHome:   dir,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))
}
