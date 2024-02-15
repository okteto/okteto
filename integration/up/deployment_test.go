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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"

	yaml "gopkg.in/yaml.v2"
)

const (
	deploymentManifestV1 = `
name: e2etest
image: python:alpine
command:
  - sh
  - -c
  - "echo -n $VAR > var.html && python -m http.server 8080"
workdir: /usr/src/app
sync:
- .:/usr/src/app
forward:
- 8083:8080
`
	deploymentManifestV2 = `
dev:
  e2etest:
    image: python:alpine
    command:
    - sh
    - -c
    - "echo -n $VAR > var.html && python -m http.server 8080"
    workdir: /usr/src/app
    sync:
    - .:/usr/src/app
    forward:
    - 8084:8080
`
	k8sManifestTemplate = `
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
        image: python:alpine
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

func TestUpDeploymentV1(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("UpDeploymentV1", user)
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

	require.NoError(t, writeFile(filepath.Join(dir, "deployment.yml"), k8sManifestTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), deploymentManifestV1))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		File:       filepath.Join(dir, "deployment.yml"),
		Name:       "e2etest",
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, commands.RunKubectlApply(kubectlBinary, kubectlOpts))
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	originalDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "e2etest", c)
	require.NoError(t, err)

	upOptions := &commands.UpOptions{
		Name:         "e2etest",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts = &commands.KubectlOptions{
		Namespace:  testNamespace,
		File:       filepath.Join(dir, "deployment.yml"),
		Name:       model.DevCloneName("e2etest"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	varLocalEndpoint := "http://localhost:8083/var.html"
	indexLocalEndpoint := "http://localhost:8083/index.html"
	indexRemoteEndpoint := fmt.Sprintf("https://e2etest-%s.%s/index.html", testNamespace, appsSubdomain)

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

	// Test modify deployment gets updated
	d, err := integration.GetDeployment(context.Background(), testNamespace, "e2etest", c)
	d.ResourceVersion = ""
	require.NoError(t, err)
	d.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	originalDeployment.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	require.NoError(t, integration.UpdateDeployment(context.Background(), testNamespace, d, c))
	require.Equal(t, "value2", integration.GetContentFromURL(varLocalEndpoint, timeout))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=e2etest", c))
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

	// Test that original hasn't change
	require.NoError(t, compareDeployment(context.Background(), originalDeployment, c))
}

func TestUpDeploymentV2(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("UpDeploymentV2", user)
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

	require.NoError(t, writeFile(filepath.Join(dir, "deployment.yml"), k8sManifestTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), deploymentManifestV2))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	kubectlOpts := &commands.KubectlOptions{
		Namespace:  testNamespace,
		File:       filepath.Join(dir, "deployment.yml"),
		Name:       "e2etest",
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, commands.RunKubectlApply(kubectlBinary, kubectlOpts))
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	originalDeployment, err := integration.GetDeployment(context.Background(), testNamespace, "e2etest", c)
	require.NoError(t, err)

	upOptions := &commands.UpOptions{
		Name:         "e2etest",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	kubectlOpts = &commands.KubectlOptions{
		Namespace:  testNamespace,
		File:       filepath.Join(dir, "deployment.yml"),
		Name:       model.DevCloneName("e2etest"),
		ConfigFile: filepath.Join(dir, ".kube", "config"),
	}
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, kubectlOpts, 1, timeout))

	varLocalEndpoint := "http://localhost:8084/var.html"
	indexLocalEndpoint := "http://localhost:8084/index.html"
	indexRemoteEndpoint := fmt.Sprintf("https://e2etest-%s.%s/index.html", testNamespace, appsSubdomain)

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

	// Test modify deployment gets updated
	d, err := integration.GetDeployment(context.Background(), testNamespace, "e2etest", c)
	d.ResourceVersion = ""
	require.NoError(t, err)
	d.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	originalDeployment.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	require.NoError(t, integration.UpdateDeployment(context.Background(), testNamespace, d, c))
	require.Equal(t, "value2", integration.GetContentFromURL(varLocalEndpoint, timeout))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=e2etest", c))
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

	// Test that original hasn't change
	require.NoError(t, compareDeployment(context.Background(), originalDeployment, c))
}

func compareDeployment(ctx context.Context, deployment *appsv1.Deployment, c kubernetes.Interface) error {
	after, err := integration.GetDeployment(ctx, deployment.GetNamespace(), deployment.GetName(), c)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(deployment.Spec)
	if err != nil {
		return err
	}

	a, err := yaml.Marshal(after.Spec)
	if err != nil {
		return err
	}

	if string(a) != string(b) {
		return fmt.Errorf("got:\n%s\nexpected:\n%s", string(a), string(b))
	}

	return nil
}
