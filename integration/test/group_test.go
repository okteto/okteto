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
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const oktetoManifestWithGroupedTests = `
test:
  seed:
    context: .
    image: alpine
    commands:
      - echo "SEED-FROM-MANIFEST"
  unit:
    context: .
    image: alpine
    groups: [pre-commit]
    commands:
      - echo "UNIT-FROM-MANIFEST"
  lint:
    context: .
    image: alpine
    groups: [pre-commit, release]
    commands:
      - echo "LINT-FROM-MANIFEST"
  e2e:
    context: .
    image: alpine
    groups: [long-running]
    depends_on:
      - seed
    commands:
      - echo "E2E-FROM-MANIFEST"
`

// TestOktetoTestRunsGroup validates that --group runs only the test containers that belong to the group
func TestOktetoTestRunsGroup(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithGroupedTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
		Groups:     []string{"pre-commit"},
	})
	require.NoError(t, err)

	require.Contains(t, out, "Test containers to run: lint, unit")
	require.Contains(t, out, "LINT-FROM-MANIFEST")
	require.Contains(t, out, "UNIT-FROM-MANIFEST")
	require.NotContains(t, out, "SEED-FROM-MANIFEST")
	require.NotContains(t, out, "E2E-FROM-MANIFEST")

	require.ElementsMatch(t, []string{"lint", "unit"}, getInsightsTestNames(t, dir, testNamespace))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestRunsGroupWithTestNameAndDependencies validates that --group is combined with the named
// test containers, and that the test containers they depend on run first even if they are not in the group
func TestOktetoTestRunsGroupWithTestNameAndDependencies(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithGroupedTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
		TestNames:  []string{"unit"},
		Groups:     []string{"long-running"},
	})
	require.NoError(t, err)

	seed := strings.Index(out, "SEED-FROM-MANIFEST")
	require.NotEqual(t, -1, seed)
	require.Greater(t, strings.Index(out, "E2E-FROM-MANIFEST"), seed)
	require.Contains(t, out, "UNIT-FROM-MANIFEST")
	require.NotContains(t, out, "LINT-FROM-MANIFEST")

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestGroupCannotOverrideCommands validates that --group cannot be combined with a command
// override. It fails before connecting to the cluster
func TestOktetoTestGroupCannotOverrideCommands(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.yml"), []byte(oktetoManifestWithGroupedTests), 0600))

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:     dir,
		OktetoHome:  dir,
		Token:       token,
		LogOutput:   "plain",
		Groups:      []string{"pre-commit"},
		CommandArgs: []string{"echo", "override"},
	})
	require.Error(t, err)
	require.Contains(t, out, "'--group' cannot be used when overriding the commands of a test container")
	require.NotContains(t, out, "Executing test container")
}
