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

package okteto

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

const (
	timeout                = 300 * time.Second
	deploymentManifestName = "deployment.yml"
	deploymentContent      = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autowake
spec:
  replicas: 1
  selector:
    matchLabels:
      app: autowake-deployment
  template:
    metadata:
      labels:
        app: autowake-deployment
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
  name: autowake-deployment
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: autowake
    port: 8080
  selector:
    app: autowake-deployment
`

	sfsManifestName = "sfs.yml"
	sfsContent      = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: autowake
spec:
  serviceName: autowake
  replicas: 1
  selector:
    matchLabels:
      app: autowake-sfs
  template:
    metadata:
      labels:
        app: autowake-sfs
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
  name: autowake-sfs
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP
  ports:
  - name: autowake
    port: 8080
  selector:
    app: autowake-sfs
`

	oktetoManifestV1Name    = "okteto.yml"
	oktetoManifestV1Content = `
name: autowake
image: python:alpine
command:
  - sh
  - -c
  - "echo -n $VAR > var.html && python -m http.server 8080"
forward:
  - 8080:8080
workdir: /usr/src/app
persistentVolume:
  enabled: false
`
	indexHTMLName = "index.html"

	stignoreName    = ".stignore"
	stignoreContent = "venv"
)

// TestAutoWakeFromURL tests the following scenario:
// - waking up the resource by accessing the endpoint
// - waking up all other resources on the namespace
func TestAutoWakeFromURL(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)

	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestAutoWake", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	// Prepare test environment
	require.NoError(t, writeDeployment(dir))
	require.NoError(t, writeStatefulset(dir))
	require.NoError(t, writeIndexHTML(dir))
	require.NoError(t, writeOktetoManifest(dir))
	require.NoError(t, writeStIgnore(dir))

	require.NoError(t, integration.RunKubectlApply(kubectlBinary, testNamespace, filepath.Join(dir, deploymentManifestName)))
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, testNamespace, "autowake", 1, timeout))

	require.NoError(t, integration.RunKubectlApply(kubectlBinary, testNamespace, filepath.Join(dir, sfsManifestName)))
	require.NoError(t, integration.WaitForStatefulset(kubectlBinary, testNamespace, "autowake", timeout))

	// Test endpoint is working
	autowakeURL := fmt.Sprintf("https://autowake-deployment-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))
	require.True(t, areNamespaceResourcesAwake(testNamespace, timeout))

	// Sleep namespace
	require.NoError(t, sleepNamespace(testNamespace))
	require.True(t, areNamespaceResourcesSleeping(testNamespace, timeout))

	// Wake resources from url
	require.NotEmpty(t, integration.GetContentFromURL(autowakeURL, timeout))
	require.True(t, areNamespaceResourcesAwake(testNamespace, timeout))

}

// TestAutoWakeFromURL tests the following scenario:
// - waking up the resource by running okteto up on it
// - waking up all other resources on the namespace
func TestAutoWakeFromRunningUp(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)

	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("TestAutoWake", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	// Prepare test environment
	require.NoError(t, writeDeployment(dir))
	require.NoError(t, writeStatefulset(dir))
	require.NoError(t, writeIndexHTML(dir))
	require.NoError(t, writeOktetoManifest(dir))
	require.NoError(t, writeStIgnore(dir))

	require.NoError(t, integration.RunKubectlApply(kubectlBinary, testNamespace, filepath.Join(dir, deploymentManifestName)))
	require.NoError(t, integration.WaitForDeployment(kubectlBinary, testNamespace, "autowake", 1, timeout))

	require.NoError(t, integration.RunKubectlApply(kubectlBinary, testNamespace, filepath.Join(dir, sfsManifestName)))
	require.NoError(t, integration.WaitForStatefulset(kubectlBinary, testNamespace, "autowake", timeout))

	// Sleep namespace
	require.NoError(t, sleepNamespace(testNamespace))
	require.True(t, areNamespaceResourcesSleeping(testNamespace, timeout))

	// Wake up from okteto up
	upOptions := &commands.UpOptions{
		Name:         "autowake",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
	}
	upCommand, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	require.True(t, areNamespaceResourcesAwake(testNamespace, timeout))

	downOpts := &commands.DownOptions{
		Namespace:    testNamespace,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		Workdir:      dir,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))
	require.True(t, commands.HasUpCommandFinished(upCommand.Pid.Pid))
}

func writeDeployment(dir string) error {
	deploymentManifestPath := filepath.Join(dir, deploymentManifestName)
	deploymentContentBytes := []byte(deploymentContent)
	if err := os.WriteFile(deploymentManifestPath, deploymentContentBytes, 0644); err != nil {
		return err
	}
	return nil
}

func writeStatefulset(dir string) error {
	sfsManifestPath := filepath.Join(dir, sfsManifestName)
	sfsContentBytes := []byte(sfsContent)
	if err := os.WriteFile(sfsManifestPath, sfsContentBytes, 0644); err != nil {
		return err
	}
	return nil
}

func writeIndexHTML(dir string) error {
	htmlManifestPath := filepath.Join(dir, indexHTMLName)
	htmlContentBytes := []byte("autowake")
	if err := os.WriteFile(htmlManifestPath, htmlContentBytes, 0644); err != nil {
		return err
	}
	return nil
}

func writeOktetoManifest(dir string) error {
	manifestManifestPath := filepath.Join(dir, oktetoManifestV1Name)
	manifestContentBytes := []byte(oktetoManifestV1Content)
	if err := os.WriteFile(manifestManifestPath, manifestContentBytes, 0644); err != nil {
		return err
	}
	return nil
}
func writeStIgnore(dir string) error {
	stignoreManifestPath := filepath.Join(dir, stignoreName)
	stignoreContentBytes := []byte(stignoreContent)
	if err := os.WriteFile(stignoreManifestPath, stignoreContentBytes, 0644); err != nil {
		return err
	}
	return nil
}

func sleepNamespace(namespace string) error {
	client, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	if err := client.Namespaces().SleepNamespace(context.Background(), namespace); err != nil {
		return err
	}
	return nil
}

func areNamespaceResourcesSleeping(namespace string, timeout time.Duration) bool {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	retry := 0
	for {
		select {
		case <-to.C:
			log.Printf("Resources not sleeping")
			return false
		case <-ticker.C:
			d, err := integration.GetDeployment(context.Background(), namespace, "autowake")
			if err != nil {
				if retry%10 == 0 {
					log.Printf("error getting deployment: %s", err.Error())
				}
				continue
			}

			if _, ok := d.Annotations[model.StateBeforeSleepingAnnontation]; !ok {
				if retry%10 == 0 {
					log.Printf("error deployment: not sleeping")
				}
				continue
			}
			sfs, err := integration.GetStatefulset(context.Background(), namespace, "autowake")
			if err != nil {
				if retry%10 == 0 {
					log.Printf("error getting sfs: %s", err.Error())
				}
				continue
			}
			if _, ok := sfs.Annotations[model.StateBeforeSleepingAnnontation]; !ok {
				if retry%10 == 0 {
					log.Printf("error sfs: not sleeping")
				}
				continue
			}
			return true
		}
	}
}

func areNamespaceResourcesAwake(namespace string, timeout time.Duration) bool {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	retry := 0
	for {
		select {
		case <-to.C:
			log.Printf("Resources not awake")
			return false
		case <-ticker.C:
			retry++
			dList, err := integration.GetDeploymentList(context.Background(), namespace)
			if err != nil {
				if retry%10 == 0 {
					log.Printf("error getting deployments: %s", err.Error())
				}
				continue
			}
			areDeploymentSleeping := false
			for _, d := range dList {
				if v, ok := d.Labels[model.DevLabel]; ok && v == "true" {
					continue
				}
				if *d.Spec.Replicas == 0 {
					areDeploymentSleeping = true
				}
			}
			if areDeploymentSleeping {
				if retry%10 == 0 {
					log.Printf("error deployments are sleeping")
				}
				continue
			}

			sfsList, err := integration.GetStatefulsetList(context.Background(), namespace)
			if err != nil {
				if retry%10 == 0 {
					log.Printf("error getting sfs: %s", err.Error())
				}

				continue
			}
			areStatefulsetSleeping := false
			for _, sfs := range sfsList {
				if *sfs.Spec.Replicas == 0 {
					areStatefulsetSleeping = true
				}
			}
			if areStatefulsetSleeping {
				if retry%10 == 0 {
					log.Printf("error sfs are sleeping")
				}
				continue
			}
			return true
		}
	}
}
