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

//go:build integration
// +build integration

package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
)

const (
	endpointModeComposeTemplate = `services:
  frontend:
    image: busybox:latest
    ports:
      - "8080:8080"
    deploy:
      endpoint_mode: %s
    command: ["sleep", "3600"]
`
)

// TestDeployEndpointModeVIP tests that endpoint_mode: vip creates a regular ClusterIP service
func TestDeployEndpointModeVIP(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)

	ctx := context.Background()
	okteto := integration.NewOktetoCommand()

	testName := integration.GetTestId("endpoint-mode-vip", user)
	composeContent := fmt.Sprintf(endpointModeComposeTemplate, "vip")

	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	err := os.WriteFile(composeFile, []byte(composeContent), 0644)
	require.NoError(t, err)

	// Deploy
	deployOptions := integration.GetDeployOptions(testName, composeFile)
	require.NoError(t, okteto.Deploy(ctx, deployOptions))

	// Check that the service was created with regular ClusterIP (not "None")
	k8sClient, _, err := okteto.GetK8sClient()
	require.NoError(t, err)

	svc, err := services.Get(ctx, "frontend", testName, k8sClient)
	require.NoError(t, err)
	require.NotEqual(t, "None", svc.Spec.ClusterIP)
	require.Equal(t, apiv1.ServiceTypeClusterIP, svc.Spec.Type)

	// Clean up
	destroyOptions := integration.GetDestroyOptions(testName)
	require.NoError(t, okteto.Destroy(ctx, destroyOptions))
}

// TestDeployEndpointModeDNSRR tests that endpoint_mode: dnsrr creates a headless service
func TestDeployEndpointModeDNSRR(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)

	ctx := context.Background()
	okteto := integration.NewOktetoCommand()

	testName := integration.GetTestId("endpoint-mode-dnsrr", user)
	composeContent := fmt.Sprintf(endpointModeComposeTemplate, "dnsrr")

	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	err := os.WriteFile(composeFile, []byte(composeContent), 0644)
	require.NoError(t, err)

	// Deploy
	deployOptions := integration.GetDeployOptions(testName, composeFile)
	require.NoError(t, okteto.Deploy(ctx, deployOptions))

	// Check that the service was created as headless (ClusterIP: "None")
	k8sClient, _, err := okteto.GetK8sClient()
	require.NoError(t, err)

	svc, err := services.Get(ctx, "frontend", testName, k8sClient)
	require.NoError(t, err)
	require.Equal(t, "None", svc.Spec.ClusterIP)
	require.Equal(t, apiv1.ServiceTypeClusterIP, svc.Spec.Type)

	// Clean up
	destroyOptions := integration.GetDestroyOptions(testName)
	require.NoError(t, okteto.Destroy(ctx, destroyOptions))
}

// TestDeployEndpointModeInvalid tests that invalid endpoint_mode values are rejected
func TestDeployEndpointModeInvalid(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)

	ctx := context.Background()
	okteto := integration.NewOktetoCommand()

	testName := integration.GetTestId("endpoint-mode-invalid", user)
	composeContent := fmt.Sprintf(endpointModeComposeTemplate, "invalid")

	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	err := os.WriteFile(composeFile, []byte(composeContent), 0644)
	require.NoError(t, err)

	// Deploy should fail with invalid endpoint_mode
	deployOptions := integration.GetDeployOptions(testName, composeFile)
	err = okteto.Deploy(ctx, deployOptions)
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint_mode")
	require.Contains(t, err.Error(), "invalid")
}

// TestDeployEndpointModeDefault tests that default behavior works when endpoint_mode is not specified
func TestDeployEndpointModeDefault(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)

	ctx := context.Background()
	okteto := integration.NewOktetoCommand()

	testName := integration.GetTestId("endpoint-mode-default", user)
	composeContent := `services:
  frontend:
    image: busybox:latest
    ports:
      - "8080:8080"
    command: ["sleep", "3600"]
`

	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	err := os.WriteFile(composeFile, []byte(composeContent), 0644)
	require.NoError(t, err)

	// Deploy
	deployOptions := integration.GetDeployOptions(testName, composeFile)
	require.NoError(t, okteto.Deploy(ctx, deployOptions))

	// Check that the service was created with default behavior (regular ClusterIP)
	k8sClient, _, err := okteto.GetK8sClient()
	require.NoError(t, err)

	svc, err := services.Get(ctx, "frontend", testName, k8sClient)
	require.NoError(t, err)
	require.NotEqual(t, "None", svc.Spec.ClusterIP)
	require.Equal(t, apiv1.ServiceTypeClusterIP, svc.Spec.Type)

	// Clean up
	destroyOptions := integration.GetDestroyOptions(testName)
	require.NoError(t, okteto.Destroy(ctx, destroyOptions))
}