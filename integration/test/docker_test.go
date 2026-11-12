//go:build integration
// +build integration

// Copyright 2026 The Okteto Authors
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

package test

import (
	"os"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testcontainersTestdata is the Testcontainers example shared with the build tests
const testcontainersTestdata = "../build/testdata/testcontainers"

// TestOktetoTestsWithDockerTestcontainers tests the following scenario:
// - running with `docker: true` a test container whose Maven tests use Testcontainers
// It needs an Okteto instance with the Docker daemon of BuildKit enabled (buildkit.dind.enabled)
func TestOktetoTestsWithDockerTestcontainers(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(testcontainersTestdata)))

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, &commands.KubeconfigOpts{
		OktetoHome: dir,
	}))

	testOptions := &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		NoCache:    true,
	}
	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, testOptions)
	require.NoError(t, err)
	assert.Contains(t, out, "Test container 'unit' passed")

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
