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
	chartYaml = `apiVersion: v2
name: e2etest
description: A Node + React application in Kubernetes
type: application
version: 0.1.0
appVersion: 1.0.0
icon: https://apps.okteto.com/movies/icon.png`
	valuesYaml = `api:
  replicaCount: 1
  image: python:alpine`

	appDeploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2etest
spec:
  replicas: {{ .Values.api.replicaCount }}
  selector:
    matchLabels:
      app.kubernetes.io/name: api
      app.kubernetes.io/instance: "{{ .Release.Name }}"
  template:
    metadata:
      labels:
        app.kubernetes.io/name: api
        app.kubernetes.io/instance: "{{ .Release.Name }}"
    spec:
      terminationGracePeriodSeconds: 1
      containers:
      - name: test
        image: {{ .Values.api.image }}
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
`

	appSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: e2etest
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: port
    port: 8080
  selector:
    app.kubernetes.io/name: api
    app.kubernetes.io/instance: "{{ .Release.Name }}"
`
)

// TestDeployPipelineFromHelm tests the following scenario:
// - Deploying a pipeline manifest locally from a helm chart
// - The endpoints generated are accessible
func TestDeployPipelineFromHelm(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createHelmChart(dir))

	testNamespace := integration.GetTestNamespace("DeployHelm", user)
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
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

// TestDeployFromHelmNameOK tests the following scenario:
// - Deploying a pipeline manifest with custom name not sanaitized
// - Deploying a pipeline manifest locally from a helm chart
// - The endpoints generated are accessible
func TestDeployFromHelmNameOK(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createHelmChart(dir))

	testNamespace := integration.GetTestNamespace("DeployFromHelmNameOK", user)
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
		Name:       "my custom name",
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Name:       "my custom name",
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
}

func createHelmChart(dir string) error {
	if err := os.Mkdir(filepath.Join(dir, "chart"), 0700); err != nil {
		return err
	}

	chartPath := filepath.Join(dir, "chart", "Chart.yaml")
	chartContent := []byte(chartYaml)
	if err := os.WriteFile(chartPath, chartContent, 0600); err != nil {
		return err
	}

	valuesPath := filepath.Join(dir, "chart", "values.yaml")
	valuesContent := []byte(valuesYaml)
	if err := os.WriteFile(valuesPath, valuesContent, 0600); err != nil {
		return err
	}

	if err := os.Mkdir(filepath.Join(dir, "chart", "templates"), 0700); err != nil {
		return err
	}

	appPath := filepath.Join(dir, "chart", "templates", "app.yaml")
	appContent := []byte(appDeploymentTemplate)
	if err := os.WriteFile(appPath, appContent, 0600); err != nil {
		return err
	}

	appSvcPath := filepath.Join(dir, "chart", "templates", "app-svc.yaml")
	appSvcContent := []byte(appSvcTemplate)
	if err := os.WriteFile(appSvcPath, appSvcContent, 0600); err != nil {
		return err
	}

	return nil
}
