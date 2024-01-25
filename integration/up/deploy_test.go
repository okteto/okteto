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
	"github.com/okteto/okteto/pkg/log/io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/require"
)

const (
	oktetoManifestV2Deploy = `build:
  app:
    context: app
    dockerfile: Dockerfile
    image: okteto.dev/test:1.0.0
deploy:
  - kubectl apply -f deployment.yaml
dev:
  e2etest:
    image: ${OKTETO_BUILD_APP_IMAGE}
    command: echo value1 > /usr/src/app/var.html && python -m http.server 8080
    workdir: /usr/src/app
    sync:
    - .:/usr/src/app
`
)

func TestUpWithDeploy(t *testing.T) {
	t.Setenv("OKTETO_K8S_REQUESTS_LOGGER_ENABLED", "true")
	// Prepare environment

	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestUpWithDeploy", user)
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

	require.NoError(t, writeFile(filepath.Join(dir, "deployment.yaml"), k8sManifestTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifestV2Deploy))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))
	require.NoError(t, createAppDockerfile(dir))

	upOptions := &commands.UpOptions{
		Name:       "e2etest",
		Namespace:  testNamespace,
		Workdir:    dir,
		Deploy:     true,
		OktetoHome: dir,
		Token:      token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		Name:       model.DevCloneName("e2etest"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	// Test that the app image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, model.DevCloneName("e2etest"), c)
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/test:1.0.0", okteto.GetContext().Registry, testNamespace)
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	indexRemoteEndpoint := fmt.Sprintf("https://e2etest-%s.%s/index.html", testNamespace, appsSubdomain)

	// Test that the same content is on the remote and on local endpoint
	require.Equal(t, integration.GetContentFromURL(indexRemoteEndpoint, timeout), testNamespace)

	// Test that making a change gets reflected on remote
	localupdatedContent := fmt.Sprintf("%s-updated-content", testNamespace)
	require.NoError(t, writeFile(indexPath, localupdatedContent))
	require.NoError(t, waitUntilUpdatedContent(indexRemoteEndpoint, localupdatedContent, timeout, upResult.ErrorChan))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexRemoteEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=e2etest", c))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexRemoteEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))

	k8sLogsFilePath := filepath.Join(dir, ".okteto", io.K8sLogsFileName)
	require.FileExists(t, k8sLogsFilePath)
	k8sLogs, err := os.ReadFile(k8sLogsFilePath)
	require.NoError(t, err)
	require.Contains(t, string(k8sLogs), fmt.Sprintf("running cmd: up --deploy=true --namespace=%s", testNamespace))
}

func getImageWithSHA(devImage string) string {
	reg := registry.NewOktetoRegistry(okteto.Config{})
	tag, err := reg.GetImageTagWithDigest(devImage)
	if err != nil {
		log.Printf("could not get %s from registry", devImage)
		return ""
	}
	return tag
}
