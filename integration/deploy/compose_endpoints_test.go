//go:build integration
// +build integration

// Copyright 2025 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/k8s/httproutes"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// TestDeployPipelineFromComposeWithGateway tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with Gateway mode
// - Verifies HTTPRoute is created and Ingress is not
// - The endpoints generated are accessible
func TestDeployPipelineFromComposeWithGateway(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=gateway"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "value-annotation-1")
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "value-annotation-2")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-1"], "value-label-1")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-2"], "value-label-2")

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	appVolume, err := integration.GetVolume(context.Background(), testNamespace, "data", c)
	require.NoError(t, err)
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "volume-annotation-1")
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "volume-annotation-2")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-1"], "volume-label-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-2"], "volume-label-2")

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify HTTPRoute exists and Ingress does NOT exist
	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "HTTPRoute 'nginx' should exist when using gateway mode")

	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "Ingress 'nginx' should NOT exist when using gateway mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployPipelineFromComposeWithIngress tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with Ingress mode
// - Verifies Ingress is created and HTTPRoute is not
// - The endpoints generated are accessible
func TestDeployPipelineFromComposeWithIngress(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

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

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "value-annotation-1")
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "value-annotation-2")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-1"], "value-label-1")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-2"], "value-label-2")

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	appVolume, err := integration.GetVolume(context.Background(), testNamespace, "data", c)
	require.NoError(t, err)
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "volume-annotation-1")
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "volume-annotation-2")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-1"], "volume-label-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-2"], "volume-label-2")

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify Ingress exists and HTTPRoute does NOT exist
	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "Ingress 'nginx' should exist when using ingress mode")

	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "HTTPRoute 'nginx' should NOT exist when using ingress mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployPipelineFromComposeWithVolumeMountsWithGateway tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with volume mounts and Gateway mode
// - Verifies HTTPRoute is created and Ingress is not
// - The endpoints generated are accessible
func TestDeployPipelineFromComposeWithVolumeMountsWithGateway(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nginx"), 0700))

	nginxPath := filepath.Join(dir, "nginx", "nginx.conf")
	nginxContent := []byte(nginxConf)
	require.NoError(t, os.WriteFile(nginxPath, nginxContent, 0600))
	require.NoError(t, createAppDockerfile(dir))
	composePath := filepath.Join(dir, "docker-compose.yml")
	composeContent := []byte(composeTemplateWithVolumeMount)
	require.NoError(t, os.WriteFile(composePath, composeContent, 0600))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=gateway"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the deployment for app has the expected annotations and labels
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "value-annotation-1")
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "value-annotation-2")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-1"], "value-label-1")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-2"], "value-label-2")

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	appVolume, err := integration.GetVolume(context.Background(), testNamespace, "data", c)
	require.NoError(t, err)
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "volume-annotation-1")
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "volume-annotation-2")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-1"], "volume-label-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-2"], "volume-label-2")

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify HTTPRoute exists and Ingress does NOT exist
	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "HTTPRoute 'nginx' should exist when using gateway mode")

	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "Ingress 'nginx' should NOT exist when using gateway mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployPipelineFromComposeWithVolumeMountsWithIngress tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with volume mounts and Ingress mode
// - Verifies Ingress is created and HTTPRoute is not
// - The endpoints generated are accessible
func TestDeployPipelineFromComposeWithVolumeMountsWithIngress(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nginx"), 0700))

	nginxPath := filepath.Join(dir, "nginx", "nginx.conf")
	nginxContent := []byte(nginxConf)
	require.NoError(t, os.WriteFile(nginxPath, nginxContent, 0600))
	require.NoError(t, createAppDockerfile(dir))
	composePath := filepath.Join(dir, "docker-compose.yml")
	composeContent := []byte(composeTemplateWithVolumeMount)
	require.NoError(t, os.WriteFile(composePath, composeContent, 0600))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=ingress"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the deployment for app has the expected annotations and labels
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "value-annotation-1")
	require.Equal(t, appDeployment.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "value-annotation-2")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-1"], "value-label-1")
	require.Equal(t, appDeployment.ObjectMeta.Annotations["dev.okteto.com/label-2"], "value-label-2")

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	appVolume, err := integration.GetVolume(context.Background(), testNamespace, "data", c)
	require.NoError(t, err)
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-1"], "volume-annotation-1")
	require.Equal(t, appVolume.ObjectMeta.Labels["dev.okteto.com/annotation-2"], "volume-annotation-2")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-1"], "volume-label-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-2"], "volume-label-2")

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify Ingress exists and HTTPRoute does NOT exist
	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "Ingress 'nginx' should exist when using ingress mode")

	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "HTTPRoute 'nginx' should NOT exist when using ingress mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestReDeployPipelineFromComposeWithGateway tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with Gateway mode
// - Redeploying to test idempotency
// - Verifies HTTPRoute is created and Ingress is not
// - The endpoints generated are accessible
func TestReDeployPipelineFromComposeWithGateway(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=gateway"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify HTTPRoute exists and Ingress does NOT exist
	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "HTTPRoute 'nginx' should exist when using gateway mode")

	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "Ingress 'nginx' should NOT exist when using gateway mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	// Redeploy to test idempotency
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestReDeployPipelineFromComposeWithIngress tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with Ingress mode
// - Redeploying to test idempotency
// - Verifies Ingress is created and HTTPRoute is not
// - The endpoints generated are accessible
func TestReDeployPipelineFromComposeWithIngress(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=ingress"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify Ingress exists and HTTPRoute does NOT exist
	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "Ingress 'nginx' should exist when using ingress mode")

	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "HTTPRoute 'nginx' should NOT exist when using ingress mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	// Redeploy to test idempotency
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployPipelineFromOktetoStacksWithGateway tests the following scenario:
// - Deploying a pipeline manifest locally from okteto stacks format with Gateway mode
// - Verifies HTTPRoute is created and Ingress is not
// - The endpoints generated are accessible
func TestDeployPipelineFromOktetoStacksWithGateway(t *testing.T) {
	t.Setenv("OKTETO_SUPPORT_STACKS_ENABLED", "true")

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStacksScenario(dir))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=gateway"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Equal(t, nginxDeployment.ObjectMeta.Labels["dev.okteto.com/label-1"], "value-label-1")
	require.Equal(t, nginxDeployment.ObjectMeta.Labels["dev.okteto.com/label-2"], "value-label-2")
	require.Equal(t, nginxDeployment.ObjectMeta.Annotations["dev.okteto.com/annotation-1"], "value-annotation-1")
	require.Equal(t, nginxDeployment.ObjectMeta.Annotations["dev.okteto.com/annotation-2"], "value-annotation-2")

	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	appVolume, err := integration.GetVolume(context.Background(), testNamespace, "data", c)
	require.NoError(t, err)
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/annotation-1"], "volume-annotation-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/annotation-2"], "volume-annotation-2")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-1"], "volume-label-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-2"], "volume-label-2")

	// Test that the app image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	// Verify HTTPRoute exists and Ingress does NOT exist
	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "HTTPRoute 'nginx' should exist when using gateway mode")

	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "Ingress 'nginx' should NOT exist when using gateway mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployPipelineFromOktetoStacksWithIngress tests the following scenario:
// - Deploying a pipeline manifest locally from okteto stacks format with Ingress mode
// - Verifies Ingress is created and HTTPRoute is not
// - The endpoints generated are accessible
func TestDeployPipelineFromOktetoStacksWithIngress(t *testing.T) {
	t.Setenv("OKTETO_SUPPORT_STACKS_ENABLED", "true")

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStacksScenario(dir))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=ingress"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Equal(t, nginxDeployment.ObjectMeta.Labels["dev.okteto.com/label-1"], "value-label-1")
	require.Equal(t, nginxDeployment.ObjectMeta.Labels["dev.okteto.com/label-2"], "value-label-2")
	require.Equal(t, nginxDeployment.ObjectMeta.Annotations["dev.okteto.com/annotation-1"], "value-annotation-1")
	require.Equal(t, nginxDeployment.ObjectMeta.Annotations["dev.okteto.com/annotation-2"], "value-annotation-2")

	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	appVolume, err := integration.GetVolume(context.Background(), testNamespace, "data", c)
	require.NoError(t, err)
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/annotation-1"], "volume-annotation-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/annotation-2"], "volume-annotation-2")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-1"], "volume-label-1")
	require.Equal(t, appVolume.ObjectMeta.Annotations["dev.okteto.com/label-2"], "volume-label-2")

	// Test that the app image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	// Verify Ingress exists and HTTPRoute does NOT exist
	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "Ingress 'nginx' should exist when using ingress mode")

	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "HTTPRoute 'nginx' should NOT exist when using ingress mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployComposeFromOktetoManifestWithGateway tests the following scenario:
// - Deploying a compose manifest locally from an okteto manifestv2 with Gateway mode
// - Verifies HTTPRoute is created and Ingress is not
// - The endpoints generated are accessible
func TestDeployComposeFromOktetoManifestWithGateway(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifestV2WithCompose))

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
		Variables:  []string{"OKTETO_COMPOSE_ENDPOINTS_TYPE=gateway"},
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/app:okteto", okteto.GetContext().Registry, testNamespace)
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify HTTPRoute exists and Ingress does NOT exist
	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "HTTPRoute 'nginx' should exist when using gateway mode")

	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "Ingress 'nginx' should NOT exist when using gateway mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployComposeFromOktetoManifestWithIngress tests the following scenario:
// - Deploying a compose manifest locally from an okteto manifestv2 with Ingress mode
// - Verifies Ingress is created and HTTPRoute is not
// - The endpoints generated are accessible
func TestDeployComposeFromOktetoManifestWithIngress(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifestV2WithCompose))

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

	// Test that the nginx image has been created correctly
	nginxDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	nginxImageDev := fmt.Sprintf("%s/%s/%s-nginx:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(nginxImageDev), nginxDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/app:okteto", okteto.GetContext().Registry, testNamespace)
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test that the k8s services has been created correctly
	appService, err := integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	require.Len(t, appService.Spec.Ports, 3)
	for _, p := range appService.Spec.Ports {
		require.Contains(t, []int32{8080, 8913, 2931}, p.Port)
	}
	nginxService, err := integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.NoError(t, err)
	require.Len(t, nginxService.Spec.Ports, 2)
	for _, p := range nginxService.Spec.Ports {
		require.Contains(t, []int32{80, 81}, p.Port)
	}

	// Verify Ingress exists and HTTPRoute does NOT exist
	ingressClient, err := ingresses.GetClient(c)
	require.NoError(t, err)
	_, err = ingressClient.Get(context.Background(), "nginx", testNamespace)
	require.NoError(t, err, "Ingress 'nginx' should exist when using ingress mode")

	httpRouteClient, err := httproutes.NewHTTPRouteClient(restConfig)
	require.NoError(t, err)
	_, err = httpRouteClient.Get(context.Background(), "nginx", testNamespace)
	require.Error(t, err, "HTTPRoute 'nginx' should NOT exist when using ingress mode")

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)

	_, err = integration.GetService(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
