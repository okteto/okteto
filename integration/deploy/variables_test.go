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
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
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
}
