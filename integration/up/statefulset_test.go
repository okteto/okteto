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
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"

	yaml "gopkg.in/yaml.v2"
)

const (
	statefulsetManifestV1 = `
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
- 8085:8080
`
	statefulsetManifestV2 = `
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
    - 8086:8080
`
	k8sSfsManifestTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: e2etest
spec:
  serviceName: e2etest
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
    app: e2etest`
)

func TestUpStatefulsetV1(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestUpStatefulsetV1", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "sfs.yml"), k8sSfsManifestTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), statefulsetManifestV1))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), "venv"))

	require.NoError(t, integration.RunKubectlApply(kubectlBinary, testNamespace, filepath.Join(dir, "sfs.yml")))
	require.NoError(t, integration.WaitForStatefulset(kubectlBinary, testNamespace, "e2etest", timeout))

	originalStatefulSet, err := integration.GetStatefulset(context.Background(), testNamespace, "e2etest")
	require.NoError(t, err)

	upOptions := &commands.UpOptions{
		Name:         "e2etest",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	require.NoError(t, integration.WaitForStatefulset(kubectlBinary, testNamespace, model.DevCloneName("e2etest"), timeout))

	varLocalEndpoint := "http://localhost:8085/var.html"
	indexLocalEndpoint := "http://localhost:8085/index.html"
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
	require.NoError(t, checkStignoreIsOnRemote(testNamespace, filepath.Join(dir, "okteto.yml"), oktetoPath))

	// Test modify statefulset gets updated
	sfs, err := integration.GetStatefulset(context.Background(), testNamespace, "e2etest")
	require.NoError(t, err)
	sfs.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	originalStatefulSet.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	require.NoError(t, integration.UpdateStatefulset(context.Background(), testNamespace, sfs))
	require.Equal(t, "value2", integration.GetContentFromURL(varLocalEndpoint, timeout))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=e2etest"))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace:    testNamespace,
		ManifestPath: upOptions.ManifestPath,
		Workdir:      dir,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))

	// Test that original hasn't change
	require.NoError(t, compareStatefulSet(context.Background(), originalStatefulSet))
}

func TestUpStatefulsetV2(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestUpStatefulsetV2", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "sfs.yml"), k8sSfsManifestTemplate))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), statefulsetManifestV2))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), "venv"))

	require.NoError(t, integration.RunKubectlApply(kubectlBinary, testNamespace, filepath.Join(dir, "sfs.yml")))
	require.NoError(t, integration.WaitForStatefulset(kubectlBinary, testNamespace, "e2etest", timeout))

	originalStatefulSet, err := integration.GetStatefulset(context.Background(), testNamespace, "e2etest")
	require.NoError(t, err)

	upOptions := &commands.UpOptions{
		Name:         "e2etest",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	require.NoError(t, integration.WaitForStatefulset(kubectlBinary, testNamespace, model.DevCloneName("e2etest"), timeout))

	varLocalEndpoint := "http://localhost:8086/var.html"
	indexLocalEndpoint := "http://localhost:8086/index.html"
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
	require.NoError(t, checkStignoreIsOnRemote(testNamespace, filepath.Join(dir, "okteto.yml"), oktetoPath))

	// Test modify statefulset gets updated
	sfs, err := integration.GetStatefulset(context.Background(), testNamespace, "e2etest")
	require.NoError(t, err)
	sfs.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	originalStatefulSet.Spec.Template.Spec.Containers[0].Env[0].Value = "value2"
	require.NoError(t, integration.UpdateStatefulset(context.Background(), testNamespace, sfs))
	require.Equal(t, "value2", integration.GetContentFromURL(varLocalEndpoint, timeout))

	// Test kill syncthing reconnection
	require.NoError(t, killLocalSyncthing(upResult.Pid.Pid))
	localSyncthingKilledContent := fmt.Sprintf("%s-kill-syncthing", testNamespace)
	require.NoError(t, writeFile(indexPath, localSyncthingKilledContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, localSyncthingKilledContent, timeout, upResult.ErrorChan))

	// Test destroy pod reconnection
	require.NoError(t, integration.DestroyPod(context.Background(), testNamespace, "app=e2etest"))
	destroyPodContent := fmt.Sprintf("%s-destroy-pod", testNamespace)
	require.NoError(t, writeFile(indexPath, destroyPodContent))
	require.NoError(t, waitUntilUpdatedContent(indexLocalEndpoint, destroyPodContent, timeout, upResult.ErrorChan))

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace:    testNamespace,
		ManifestPath: upOptions.ManifestPath,
		Workdir:      dir,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))

	// Test that original hasn't change
	require.NoError(t, compareStatefulSet(context.Background(), originalStatefulSet))
}

func compareStatefulSet(ctx context.Context, deployment *appsv1.StatefulSet) error {
	after, err := integration.GetStatefulset(ctx, deployment.GetNamespace(), deployment.GetName())
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
