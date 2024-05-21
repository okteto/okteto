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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

var (
	oktetoManifestWithEnvContent = `
deploy:
  - echo "DEPLOY_MASKED_VAR=${EXTERNAL_VARIABLE}"
  - echo "DEPLOY_UNMASKED_SHORT_VAR=${EXTERNAL_BOOL_VARIABLE}"
destroy:
  - echo "DESTROY_MASKED_VAR=${EXTERNAL_VARIABLE}"
  - echo "DESTROY_UNMASKED_SHORT_VAR=${EXTERNAL_BOOL_VARIABLE}"`

	dotEnvContent = `
CLI_TEST_MY_VAR1=.env
CLI_TEST_MY_VAR2=.env
CLI_TEST_MY_VAR3=.env
CLI_TEST_MY_VAR4=.env
`
	oktetoManifestWithVars = `
deploy:
  commands:
  - printenv
  
destroy:
  commands:
  - printenv
`
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

	manifestPath := filepath.Join(dir, oktetoManifestName)
	err = os.WriteFile(manifestPath, []byte(oktetoManifestWithEnvContent), 0600)
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("DeployDestroyVars", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	variables := []string{
		"EXTERNAL_VARIABLE=123456",
		"EXTERNAL_BOOL_VARIABLE=false",
	}

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Variables:  variables,
	}
	o, err := commands.RunOktetoDeployAndGetOutput(oktetoPath, deployOptions)
	require.NoError(t, err)

	require.Equal(t, true, strings.Contains(o, "DEPLOY_MASKED_VAR=***"))
	require.Equal(t, true, strings.Contains(o, "DEPLOY_UNMASKED_SHORT_VAR=false"))

	ctx := context.Background()
	cfgMap, err := integration.GetConfigmap(ctx, testNamespace, "okteto-git-001", c)
	require.NoError(t, err)

	require.NotNil(t, cfgMap)
	require.NotNil(t, cfgMap.Data)
	require.NotEmpty(t, cfgMap.Data["variables"])

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}

	o, err = commands.RunOktetoDestroyAndGetOutput(oktetoPath, destroyOptions)
	require.NoError(t, err)

	require.Equal(t, true, strings.Contains(o, "DESTROY_MASKED_VAR=***"))
	require.Equal(t, true, strings.Contains(o, "DESTROY_UNMASKED_SHORT_VAR=false"))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestDeployVariablesOrder validates the order of precedence of the different Okteto Variable types as documented in:
// https://www.okteto.com/docs/1.20/core/okteto-variables/#types-of-variables
// Note: this test requires two variables configured in the Okteto UI Admin Variables:
// 'CLI_TEST_MY_VAR4=admin' and 'CLI_TEST_MY_VAR5=admin'
func TestDeployVariablesOrder(t *testing.T) {
	t.Setenv("CLI_TEST_MY_VAR3", "local")

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace("DeployVariablesOrder", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	manifestPath := filepath.Join(dir, oktetoManifestName)
	manifestContent := []byte(oktetoManifestWithVars)
	err = os.WriteFile(manifestPath, manifestContent, 0600)
	require.NoError(t, err)

	dotEnvPath := filepath.Join(dir, ".env")
	err = os.WriteFile(dotEnvPath, []byte(dotEnvContent), 0600)
	require.NoError(t, err)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Variables: []string{
			"CLI_TEST_MY_VAR1=flag",
		},
	}

	deployOutput, deployErr := commands.RunOktetoDeployAndGetOutput(oktetoPath, deployOptions)

	require.NoError(t, deployErr)
	require.Contains(t, deployOutput, "CLI_TEST_MY_VAR1=flag")
	require.Contains(t, deployOutput, "CLI_TEST_MY_VAR2=.env")
	require.Contains(t, deployOutput, "CLI_TEST_MY_VAR3=local")
	require.Contains(t, deployOutput, "CLI_TEST_MY_VAR4=.env")
	require.Contains(t, deployOutput, "CLI_TEST_MY_VAR5=admin")
	require.Contains(t, deployOutput, "Okteto Variable 'CLI_TEST_MY_VAR4' is overridden by a local environment variable with the same name")

	// reset all values to make sure destroy is not affected by what was set in deploy
	os.Unsetenv("CLI_TEST_MY_VAR1")
	os.Unsetenv("CLI_TEST_MY_VAR2")
	os.Unsetenv("CLI_TEST_MY_VAR3")
	os.Unsetenv("CLI_TEST_MY_VAR4")
	os.Unsetenv("CLI_TEST_MY_VAR5")

	// setting again the local variable needed for destroy
	t.Setenv("CLI_TEST_MY_VAR3", "local")

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	destroyOutput, destroyErr := commands.RunOktetoDestroyAndGetOutput(oktetoPath, destroyOptions)
	require.NoError(t, destroyErr)
	require.Contains(t, destroyOutput, "CLI_TEST_MY_VAR1=flag")
	require.Contains(t, destroyOutput, "CLI_TEST_MY_VAR2=.env")
	require.Contains(t, destroyOutput, "CLI_TEST_MY_VAR3=local")
	require.Contains(t, destroyOutput, "CLI_TEST_MY_VAR4=.env")
	require.Contains(t, destroyOutput, "CLI_TEST_MY_VAR5=admin")
	require.Contains(t, deployOutput, "Okteto Variable 'CLI_TEST_MY_VAR4' is overridden by a local environment variable with the same name")

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
