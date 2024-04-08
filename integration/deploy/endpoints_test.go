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
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	oktetoManifestWithInferredNameEndpointsContent = `build:
  app:
    context: app
deploy:
  commands:
    - kubectl apply -f k8s.yml
  endpoints:
    - path: /
      service: e2etest
      port: 8080
`

	oktetoManifestWithNameEndpointsContent = `build:
  app:
    context: app
deploy:
  commands:
    - kubectl apply -f k8s.yml
  endpoints:
    myname:
      - path: /
        service: e2etest
        port: 8080
`

	stackWithEndpointsInferredNameContent = `endpoints:
  - path: /
    service: nginx
    port: 80
services:
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

	stackWithEndpointsNameContent = `endpoints:
  myname:
    - path: /
      service: nginx
      port: 80
services:
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

	oktetoManifestV2WithComposeWithEndpoints = `build:
  app:
    context: app
    image: okteto.dev/app:okteto
deploy:
  compose: okteto-stack.yml
  endpoints:
      manif:
        - path: /
          service: nginx
          port: 80`
)

// Test_EndpointsFromOktetoManifest_InferredName tests the following scenario:
// - Deploying a okteto manifest locally
// - K8s Service manifest has auto-ingress, endpoints are automatically created when kubectl apply
// - The endpoints declared at the manifest deploy section are accessible and have the inferred name from repo
func Test_EndpointsFromOktetoManifest_InferredName(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createEndpointsFromOktetoManifestInferredName(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("EndpointManifestInfer", user)
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

	// Test that endpoint works - created by auto-ingress at k8s manifest
	autoIngressURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autoIngressURL, timeout))

	// Test that endpoint works - created by okteto manifest
	manifestEndpointURL := fmt.Sprintf("https://001-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(manifestEndpointURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

func createEndpointsFromOktetoManifestInferredName(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestWithInferredNameEndpointsContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

// Test_EndpointsFromOktetoManifest_Name tests the following scenario:
// - Deploying a okteto manifest locally
// - K8s Service manifest has auto-ingress, endpoints are automatically created when kubectl apply
// - The endpoints declared at the manifest deploy section are accessible and have the specified name
func Test_EndpointsFromOktetoManifest_Name(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createEndpointsFromOktetoManifestName(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("EndpointsManifest", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "info",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that endpoint works - created by auto-ingress at k8s manifest
	autoIngressURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autoIngressURL, timeout))

	// Test that endpoint works - created by okteto manifest
	manifestEndpointURL := fmt.Sprintf("https://myname-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(manifestEndpointURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}

func createEndpointsFromOktetoManifestName(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestWithNameEndpointsContent)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}

// Test_EndpointsFromStackWithEndpoints_InferredName tests the following scenario:
// - Deploying a stack manifest locally from a compose file with endpoints section
// - The endpoints generated are accessible and name is inferred from repo
func Test_EndpointsFromStackWith_InferredName(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStackWithEndpointsInferredNameScenario(dir))

	testNamespace := integration.GetTestNamespace("TEndpointsFromStackInfer", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "info",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://001-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}

func createStackWithEndpointsInferredNameScenario(dir string) error {
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
	composeContent := []byte(stackWithEndpointsInferredNameContent)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}

	return nil
}

// TestDeployPipelineFromOktetoStacksWithEndpointsName tests the following scenario:
// - Deploying a pipeline manifest locally from a compose file with endpoints section (stack)
// - The endpoints generated are accessible and name is custom
func Test_EndpointsFromStack_Name(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStackWithEndpointsNameScenario(dir))

	testNamespace := integration.GetTestNamespace("TEndpointsFromStack", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://myname-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}

func createStackWithEndpointsNameScenario(dir string) error {
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
	composeContent := []byte(stackWithEndpointsNameContent)
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		return err
	}

	return nil
}

// TestDeployPipelineFromCompose tests the following scenario:
// - Deploying a compose manifest locally from an okteto manifestv2
// - The endpoints generated are the ones at the manifest
func Test_EndpointsFromStackAndManifest(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStackWithEndpointsNameScenario(dir))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifestV2WithComposeWithEndpoints))

	testNamespace := integration.GetTestNamespace("EndpointsStackAndManif", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://manif-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}

// Test_EndpointsFromOktetoManifest_NameOption tests the following scenario:
// - Deploying a okteto manifest locally
// - K8s Service manifest has auto-ingress, endpoints are automatically created when kubectl apply
// - The endpoints declared at the manifest deploy section are accessible and have name of deploy option
func Test_EndpointsFromOktetoManifest_NameOption(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createEndpointsFromOktetoManifestInferredName(dir))
	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("EndpointManifestNameOp", user)
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
		Name:       "my test",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that endpoint works - created by auto-ingress at k8s manifest
	autoIngressURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autoIngressURL, timeout))

	// Test that endpoint works - created by okteto manifest
	manifestEndpointURL := fmt.Sprintf("https://my-test-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(manifestEndpointURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Name:       "my test",
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

// Test_EndpointsFromStackWithEndpoints_InferredName tests the following scenario:
// - Deploying a stack manifest locally from a compose file with endpoints section
// - The endpoints generated are accessible and name from deploy option
func Test_EndpointsFromStackWith_NameOption(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createStackWithEndpointsInferredNameScenario(dir))

	testNamespace := integration.GetTestNamespace("TEpStackNameOpt", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Name:       "my test",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test endpoints are accessible
	nginxURL := fmt.Sprintf("https://my-test-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(nginxURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Name:       "my test",
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}
