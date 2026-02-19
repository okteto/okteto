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
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// TestDeployIngress tests okteto deploy with Ingress
// It verifies that when using ingress mode, Ingress resources are created instead of HTTPRoute
func TestDeployIngress(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createDeployComposeScenario(dir))

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
	c, restConfig, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "info",
		Wait:       false,
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=ingress"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the deployments have been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.NotNil(t, nginxDeployment)

	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.NotNil(t, appDeployment)

	// Test that the k8s services have been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.NotEmpty(t, appService.Spec.Ports)

	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.NotEmpty(t, nginxService.Spec.Ports)

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

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))
	log.Printf("✓ Endpoint %s is accessible", nginxURL)

	// Test destroy
	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	// Verify nginx service is destroyed
	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err), "nginx service should be destroyed")

	// Cleanup namespace
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
