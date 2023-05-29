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
	"errors"
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
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

var (
	oktetoManifestWithEnvContent = `deploy:
  - echo "%s ${EXTERNAL_VARIABLE}"
destroy:
- echo "%s ${EXTERNAL_VARIABLE}"`
)

const (
	echoDeployMessage  = "printing deploy external variable..."
	echoDestroyMessage = "printing destroy external variable..."
)

// TestDeployAndDestroyOktetoManifestWithEnv tests the following scenario:
// - Deploying a okteto manifest locally with masked variables
// - Destroying a okteto manifest locally with masked variables
func TestDeployAndDestroyOktetoManifestWithEnv(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createOktetoManifestwithEnv(dir))

	testNamespace := integration.GetTestNamespace("TestDeployDestroyVars", user)
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

	variables := "EXTERNAL_VARIABLE=test"

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Variables:  variables,
	}
	o, err := commands.RunOktetoDeployAndGetOutput(oktetoPath, deployOptions)
	require.NoError(t, err)

	err = expectMaskedVariableAtDeploy(o)
	require.NoError(t, err)

	ctx := context.Background()
	cfgMap, err := integration.GetConfigmap(ctx, testNamespace, "okteto-git-001", c)
	require.NoError(t, err)

	err = expectConfigMapToIncludeVariables(cfgMap)
	require.NoError(t, err)

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	o, err = commands.RunOktetoDestroyAndGetOutput(oktetoPath, destroyOptions)
	require.NoError(t, err)

	err = expectMaskedVariableAtDestroy(o)
	require.NoError(t, err)
}

func expectMaskedVariableAtDeploy(o string) error {
	if ok := strings.Contains(o, fmt.Sprintf("%s ***", echoDeployMessage)); !ok {
		log.Print(o)
		return errors.New("external variable at deploy is not being masked")
	}
	return nil
}

func expectMaskedVariableAtDestroy(o string) error {
	if ok := strings.Contains(o, fmt.Sprintf("%s ***", echoDestroyMessage)); !ok {
		log.Print(o)
		return errors.New("external variable at destroy is not being masked")
	}
	return nil
}

func expectConfigMapToIncludeVariables(cfgmap *v1.ConfigMap) error {
	if cfgmap == nil {
		return errors.New("configmap not found")
	}

	_, ok := cfgmap.Data["variables"]
	if !ok {
		return errors.New("config map does not have variables")
	}

	return nil
}

func createOktetoManifestwithEnv(dir string) error {
	manifestPath := filepath.Join(dir, oktetoManifestName)
	manifestContent := []byte(fmt.Sprintf(oktetoManifestWithEnvContent, echoDeployMessage, echoDestroyMessage))
	if err := os.WriteFile(manifestPath, manifestContent, 0600); err != nil {
		return err
	}
	return nil
}
