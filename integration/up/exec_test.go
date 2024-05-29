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
	"log"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

func TestExecAutocreate(t *testing.T) {
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("execAutocreate", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	indexPath := filepath.Join(dir, "index.html")
	require.NoError(t, writeFile(indexPath, testNamespace))
	log.Printf("original 'index.html' content: %s", testNamespace)

	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), autocreateManifestV2))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	upOptions := &commands.UpOptions{
		Name:         "autocreate",
		Namespace:    testNamespace,
		Workdir:      dir,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	execOpts := &commands.ExecOptions{
		Namespace:    testNamespace,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
		Command:      []string{"echo", testNamespace},
		Service:      "autocreate",
	}

	otuput, err := commands.RunExecCommand(oktetoPath, execOpts)

	require.NoError(t, err)
	require.Contains(t, otuput, testNamespace)
	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))
}

func TestExec(t *testing.T) {
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
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

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

	execOpts := &commands.ExecOptions{
		Namespace:    testNamespace,
		ManifestPath: filepath.Join(dir, "okteto.yml"),
		OktetoHome:   dir,
		Token:        token,
		Command:      []string{"echo", testNamespace},
		Service:      "e2etest",
	}

	otuput, err := commands.RunExecCommand(oktetoPath, execOpts)

	require.NoError(t, err)
	require.Contains(t, otuput, testNamespace)
	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))
}
