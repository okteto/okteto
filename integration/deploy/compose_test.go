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

const (
	composeTemplate = `services:
  app:
    build: app
    entrypoint: python -m http.server 8080
    ports:
      - 8080
      - 8913
    labels:
      dev.okteto.com/policy: keep
      dev.okteto.com/label-1: value-label-1
      dev.okteto.com/label-2: value-label-2
    annotations:
      dev.okteto.com/annotation-1: value-annotation-1
      dev.okteto.com/annotation-2: value-annotation-2
  nginx:
    build: nginx
    volumes:
      - ./nginx/nginx.conf:/tmp/nginx.conf
    entrypoint: /bin/bash -c "envsubst < /tmp/nginx.conf > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"
    environment:
      - FLASK_SERVER_ADDR=app:8080
    ports:
      - 81:80
    depends_on:
      app:
        condition: service_started
    container_name: web-svc
    healthcheck:
      test: service nginx status || exit 1
      interval: 45s
      timeout: 5m
      retries: 5
      start_period: 30s
  db:
    image: alpine
    volumes:
      - data:/data
    labels:
      dev.okteto.com/label-1: statefulset-label-1
      dev.okteto.com/label-2: statefulset-label-2
    annotations:
      dev.okteto.com/annotation-1: statefulset-annotation-1
      dev.okteto.com/annotation-2: statefulset-annotation-2
volumes:
  data:
    labels:
      dev.okteto.com/label-1: volume-label-1
      dev.okteto.com/label-2: volume-label-2
    annotations:
      dev.okteto.com/annotation-1: volume-annotation-1
      dev.okteto.com/annotation-2: volume-annotation-2`

	composeTemplateWithVolumeMount = `services:
  app:
    build: app
    entrypoint: python -m http.server 8080
    ports:
      - 8080
      - 8913
    labels:
      dev.okteto.com/policy: keep
      dev.okteto.com/label-1: value-label-1
      dev.okteto.com/label-2: value-label-2
    annotations:
      dev.okteto.com/annotation-1: value-annotation-1
      dev.okteto.com/annotation-2: value-annotation-2
  nginx:
    image: nginx:latest
    volumes:
      - ./nginx/nginx.conf:/tmp/nginx.conf
    entrypoint: /bin/bash -c "envsubst < /tmp/nginx.conf > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"
    environment:
      - FLASK_SERVER_ADDR=app:8080
    ports:
      - 81:80
    depends_on:
      app:
        condition: service_started
    container_name: web-svc
    healthcheck:
      test: service nginx status || exit 1
      interval: 45s
      timeout: 5m
      retries: 5
      start_period: 30s
  db:
    image: alpine
    volumes:
      - data:/data
    labels:
      dev.okteto.com/label-1: statefulset-label-1
      dev.okteto.com/label-2: statefulset-label-2
    annotations:
      dev.okteto.com/annotation-1: statefulset-annotation-1
      dev.okteto.com/annotation-2: statefulset-annotation-2
volumes:
  data:
    labels:
      dev.okteto.com/label-1: volume-label-1
      dev.okteto.com/label-2: volume-label-2
    annotations:
      dev.okteto.com/annotation-1: volume-annotation-1
      dev.okteto.com/annotation-2: volume-annotation-2`

	stacksTemplate = `services:
  app:
    build: app
    command: python -m http.server 8080
    ports:
    - 8080
    environment:
    - RABBITMQ_PASS
  nginx:
    build: nginx
    volumes:
    - ./nginx/nginx.conf:/tmp/nginx.conf
    command: /bin/bash -c "envsubst < /tmp/nginx.conf > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"
    environment:
    - FLASK_SERVER_ADDR=app:8080
    public: true
    ports:
    - 80
    depends_on:
      app:
        condition: service_started
    container_name: web-svc
    labels:
      dev.okteto.com/label-1: value-label-1
      dev.okteto.com/label-2: value-label-2
    annotations:
      dev.okteto.com/annotation-1: value-annotation-1
      dev.okteto.com/annotation-2: value-annotation-2
    healthcheck:
      test: service nginx status || exit 1
      interval: 45s
      timeout: 5m
      retries: 5
      start_period: 30s
  db:
    image: alpine
    volumes:
      - data:/data
    labels:
      dev.okteto.com/label-1: statefulset-label-1
      dev.okteto.com/label-2: statefulset-label-2
    annotations:
      dev.okteto.com/annotation-1: statefulset-annotation-1
      dev.okteto.com/annotation-2: statefulset-annotation-2
volumes:
  data:
    labels:
      dev.okteto.com/label-1: volume-label-1
      dev.okteto.com/label-2: volume-label-2
    annotations:
      dev.okteto.com/annotation-1: volume-annotation-1
      dev.okteto.com/annotation-2: volume-annotation-2`
	appDockerfile = `FROM python:alpine
EXPOSE 2931`
	nginxConf = `server {
  listen 80;
  location / {
    proxy_pass http://$FLASK_SERVER_ADDR;
  }
}`
	nginxDockerfile = `FROM nginx
COPY ./nginx.conf /tmp/nginx.conf
`

	oktetoManifestV2WithCompose = `build:
  app:
    context: app
    image: okteto.dev/app:okteto
deploy:
  compose: docker-compose.yml
`
)

// TestDeployPipelineFromCompose tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
func TestDeployPipelineFromCompose(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

	testNamespace := integration.GetTestNamespace("DeployCompose", user)
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
		LogOutput:  "info",
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
}

// TestDeployPipelineFromComposeWithVolumeMounts tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with a service specifying an image and volume mounts
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
// - Image is generated with the volume mounts
func TestDeployPipelineFromComposeWithVolumeMounts(t *testing.T) {
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

	testNamespace := integration.GetTestNamespace("DeployComposeWithVolMount", user)
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
		LogOutput:  "info",
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
}

// TestReDeployPipelineFromCompose tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
func TestReDeployPipelineFromCompose(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

	testNamespace := integration.GetTestNamespace("ReDeployCompose", user)
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
		LogOutput:  "info",
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

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://nginx-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

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
}

// TestDeployPipelineFromComposeOnlyOneSvc tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
func TestDeployPipelineFromComposeOnlyOneSvc(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

	testNamespace := integration.GetTestNamespace("DeployPartialCompose", user)
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
		Workdir:          dir,
		ServicesToDeploy: []string{"app"},
		Namespace:        testNamespace,
		OktetoHome:       dir,
		Token:            token,
		LogOutput:        "info",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that the nginx image has been created correctly
	_, err = integration.GetDeployment(context.Background(), testNamespace, "nginx", c)
	require.True(t, k8sErrors.IsNotFound(err))

	// Test that the nginx image has been created correctly
	appDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "app", c)
	require.NoError(t, err)
	appImageDev := fmt.Sprintf("%s/%s/%s-app:okteto", okteto.GetContext().Registry, testNamespace, filepath.Base(dir))
	require.Equal(t, getImageWithSHA(appImageDev), appDeployment.Spec.Template.Spec.Containers[0].Image)

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
}

// TestDeployPipelineFromOktetoStacks tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
func TestDeployPipelineFromOktetoStacks(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStacksScenario(dir))

	testNamespace := integration.GetTestNamespace("DeployStacks", user)
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
		LogOutput:  "info",
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
}

// TestDeployPipelineFromCompose tests the following scenario:
// - Deploying a compose manifest locally from an okteto manifestv2
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
func TestDeployComposeFromOktetoManifest(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifestV2WithCompose))

	testNamespace := integration.GetTestNamespace("DeployComposeManifest", user)
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
		LogOutput:  "info",
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
}
func TestComposeDependsOnNonExistingService(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	composePath := filepath.Join(dir, "docker-compose.yml")
	composeContent := []byte(`
services:
  app:
    image: alpine
    depends_on:
      - nginx
`)
	err = os.WriteFile(composePath, composeContent, 0600)
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("NotPanic", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	output, err := commands.GetOktetoDeployCmdOutput(oktetoPath, deployOptions)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(string(output)), "invalid depends_on: service 'app' depends on service 'nginx' which is undefined")

	deployOptionsWithFile := &commands.DeployOptions{
		Workdir:      dir,
		Namespace:    testNamespace,
		OktetoHome:   dir,
		Token:        token,
		ManifestPath: "docker-compose.yml",
	}
	output, err = commands.GetOktetoDeployCmdOutput(oktetoPath, deployOptionsWithFile)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(string(output)), "invalid depends_on: service 'app' depends on service 'nginx' which is undefined")
}

func createComposeScenario(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "nginx"), 0700); err != nil {
		return err
	}

	nginxPath := filepath.Join(dir, "nginx", "nginx.conf")
	nginxContent := []byte(nginxConf)
	if err := os.WriteFile(nginxPath, nginxContent, 0600); err != nil {
		return err
	}

	if err := createNginxDockerfile(dir); err != nil {
		return err
	}

	if err := createAppDockerfile(dir); err != nil {
		return err
	}

	composePath := filepath.Join(dir, "docker-compose.yml")
	composeContent := []byte(composeTemplate)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}

	return nil
}

func createStacksScenario(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "nginx"), 0700); err != nil {
		return err
	}

	nginxPath := filepath.Join(dir, "nginx", "nginx.conf")
	nginxContent := []byte(nginxConf)
	if err := os.WriteFile(nginxPath, nginxContent, 0600); err != nil {
		return err
	}

	if err := createNginxDockerfile(dir); err != nil {
		return err
	}

	if err := createAppDockerfile(dir); err != nil {
		return err
	}

	composePath := filepath.Join(dir, "okteto-stack.yml")
	composeContent := []byte(stacksTemplate)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}

	return nil
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

func createAppDockerfile(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "app"), 0700); err != nil {
		return err
	}

	appDockerfilePath := filepath.Join(dir, "app", "Dockerfile")
	appDockerfileContent := []byte(appDockerfile)
	if err := os.WriteFile(appDockerfilePath, appDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

func createNginxDockerfile(dir string) error {
	nginxDockerfilePath := filepath.Join(dir, "nginx", "Dockerfile")
	nginxDockerfileContent := []byte(nginxDockerfile)
	if err := os.WriteFile(nginxDockerfilePath, nginxDockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}
