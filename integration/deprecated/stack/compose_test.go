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

package stack

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/require"
)

const (
	composeTemplate = `services:
  app:
    build: app
    entrypoint: echo -n $RABBITMQ_PASS > var.html && python -m http.server 8080
    ports:
      - 8080
      - 8913
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
      start_period: 30s`
	stacksTemplate = `services:
  app:
    build: app
    command: echo -n $RABBITMQ_PASS > var.html && python -m http.server 8080
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
    healthcheck:
      test: service nginx status || exit 1
      interval: 45s
      timeout: 5m
      retries: 5
      start_period: 30s`
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
)

// TestDeployPipelineFromCompose tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
func TestDeployPipelineFromCompose(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, createComposeScenario(dir))

	testNamespace := integration.GetTestNamespace("PipeCompose", user)
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

	deployOptions := &commands.StackDeployOptions{
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoStackDeploy(oktetoPath, deployOptions))

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

	destroyOptions := &commands.StackDestroyOptions{
		Workdir:    dir,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoStackDestroy(oktetoPath, destroyOptions))
}

// TestDeployPipelineFromOktetoStacks tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file
// - The endpoints generated are accessible
// - Depends on
// - Test secret injection
// - Test that port from image is imported
func TestDeployPipelineFromOktetoStacks(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, createStacksScenario(dir))

	testNamespace := integration.GetTestNamespace("PipeStacks", user)
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

	deployOptions := &commands.StackDeployOptions{
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoStackDeploy(oktetoPath, deployOptions))

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

	destroyOptions := &commands.StackDestroyOptions{
		Workdir:    dir,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoStackDestroy(oktetoPath, destroyOptions))
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
