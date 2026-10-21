//go:build integration
// +build integration

// Copyright 2023-2025 The Okteto Authors
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

package gateway

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/httproutes"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

// TestUpIngress tests okteto up with compose using Ingress
// It verifies that when OKTETO_COMPOSE_ENDPOINTS_TYPE=ingress, Ingress resources are created instead of HTTPRoute
// Note: Cannot use t.Parallel() because t.Setenv() modifies process-wide environment
func TestUpIngress(t *testing.T) {
	// Set environment variable to force Ingress
	t.Setenv("OKTETO_COMPOSE_ENDPOINTS_TYPE", "ingress")

	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, &commands.KubeconfigOpts{
		OktetoHome: dir,
	}))

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "docker-compose.yml"), composeTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createNginxDir(dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	c, restConfig, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	originalDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	upOptions := &commands.UpOptions{
		Name:       "app",
		Namespace:  testNamespace,
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		Name:       model.DevCloneName("app"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	// Verify Ingress exists
	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)

	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "Ingress 'nginx' should exist when using ingress mode")
	log.Printf("✓ Ingress 'nginx' exists in namespace '%s'", testNamespace)

	// Verify HTTPRoute does NOT exist
	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)

	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "HTTPRoute 'nginx' should NOT exist when using ingress mode")
	log.Printf("✓ HTTPRoute 'nginx' does not exist (expected)")

	varRemoteEndpoint := fmt.Sprintf("https://nginx-%s.%s/var.html", testNamespace, appsSubdomain)
	indexRemoteEndpoint := fmt.Sprintf("https://nginx-%s.%s/index.html", testNamespace, appsSubdomain)

	// Test that environment variable is injected correctly
	require.Contains(t, integration.GetContentFromURL(varRemoteEndpoint, timeout), "value1")

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
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "stack.okteto.com/service=app", c))
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

	// Test that original hasn't change
	require.NoError(t, compareDeployment(context.Background(), originalDeployment, c))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
