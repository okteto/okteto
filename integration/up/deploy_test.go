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

package up

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
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
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestUpWithDeploy", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "deployment.yaml"), k8sManifestTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifestV2Deploy))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), "venv"))
	require.NoError(t, createAppDockerfile(dir))

	upOptions := &commands.UpOptions{
		Name:      "e2etest",
		Namespace: testNamespace,
		Workdir:   dir,
		Deploy:    true,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	require.NoError(t, integration.WaitForDeployment(kubectlBinary, testNamespace, model.DevCloneName("e2etest"), 1, timeout))

	// Test that the app image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, model.DevCloneName("e2etest"))
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/test:1.0.0", okteto.Context().Registry, testNamespace)
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
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=e2etest"))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexRemoteEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))

}

func getImageWithSHA(devImage string) string {
	reg := registry.NewOktetoRegistry()
	tag, err := reg.GetImageTagWithDigest(devImage)
	if err != nil {
		log.Printf("could not get %s from registry", devImage)
		return ""
	}
	return tag
}
