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

package okteto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const manifestWithDependencies = `
deploy:
  - echo "dependency variable ${OKTETO_DEPENDENCY_POSTGRESQL_VARIABLE_TEST_VARIABLE}"
dependencies:
  postgresql:
    repository: https://github.com/okteto/movies
    branch: cli-e2e
    wait: true
    variables:
      TEST_VARIABLE: test-value
`

const remoteManifestWithDependencies = `
deploy:
  remote: true
  commands:
    - echo "dependency variable ${OKTETO_DEPENDENCY_POSTGRESQL_VARIABLE_TEST_VARIABLE}"
dependencies:
  postgresql:
    repository: https://github.com/okteto/movies
    branch: cli-e2e
    wait: true
    variables:
      TEST_VARIABLE: test-value
`

func TestDependencies(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace := integration.GetTestNamespace("Dependency", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	require.NoError(t, createDependenciesManifest(dir, manifestWithDependencies))
	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	output, err := commands.GetOktetoDeployCmdOutput(oktetoPath, deployOptions)
	require.NoError(t, err, "there was an error executing the command, output is: %s", string(output))

	expectedOutputCommand := "dependency variable test-value"
	require.Contains(t, strings.ToLower(string(output)), expectedOutputCommand)

	contentURL := fmt.Sprintf("https://movies-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(contentURL, timeout))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

func TestDependenciesOnRemote(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace := integration.GetTestNamespace("RemoteDep", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	require.NoError(t, createDependenciesManifest(dir, remoteManifestWithDependencies))
	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	output, err := commands.GetOktetoDeployCmdOutput(oktetoPath, deployOptions)
	require.NoError(t, err, "there was an error executing the command, output is: %s", string(output))

	expectedOutputCommand := "dependency variable test-value"
	require.Contains(t, strings.ToLower(string(output)), expectedOutputCommand)

	contentURL := fmt.Sprintf("https://movies-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(contentURL, timeout))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

func createDependenciesManifest(dir, manifest string) error {
	if err := os.Mkdir(filepath.Join(dir, "nginx"), 0700); err != nil {
		return err
	}

	manifestPath := filepath.Join(dir, "okteto.yml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0600); err != nil {
		return err
	}
	return nil
}
