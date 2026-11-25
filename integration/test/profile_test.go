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

const oktetoManifestWithProfiledTests = `
test:
  seed:
    context: .
    image: alpine
    commands:
      - echo "SEED-FROM-MANIFEST"
  unit:
    context: .
    image: alpine
    profiles: [pre-commit]
    commands:
      - echo "UNIT-FROM-MANIFEST"
  lint:
    context: .
    image: alpine
    profiles: [pre-commit, release]
    commands:
      - echo "LINT-FROM-MANIFEST"
  e2e:
    context: .
    image: alpine
    profiles: [long-running]
    depends_on:
      - seed
    commands:
      - echo "E2E-FROM-MANIFEST"
`

// TestOktetoTestRunsTestsWithoutProfiles validates that okteto test only runs the test containers
// without profiles
func TestOktetoTestRunsTestsWithoutProfiles(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithProfiledTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
	})
	require.NoError(t, err)

	require.Contains(t, out, "Test containers to run: seed")
	require.Contains(t, out, "SEED-FROM-MANIFEST")
	require.NotContains(t, out, "UNIT-FROM-MANIFEST")
	require.NotContains(t, out, "LINT-FROM-MANIFEST")
	require.NotContains(t, out, "E2E-FROM-MANIFEST")

	require.ElementsMatch(t, []string{"seed"}, getInsightsTestNames(t, dir, testNamespace))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestRunsProfile validates that --profile runs the test containers of the profile along
// with the ones without profiles
func TestOktetoTestRunsProfile(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithProfiledTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
		Profiles:   []string{"pre-commit"},
	})
	require.NoError(t, err)

	require.Contains(t, out, "Test containers to run: lint, seed, unit")
	require.Contains(t, out, "LINT-FROM-MANIFEST")
	require.Contains(t, out, "SEED-FROM-MANIFEST")
	require.Contains(t, out, "UNIT-FROM-MANIFEST")
	require.NotContains(t, out, "E2E-FROM-MANIFEST")

	require.ElementsMatch(t, []string{"lint", "seed", "unit"}, getInsightsTestNames(t, dir, testNamespace))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestProfilesFromEnvOverrideFlag validates that the OKTETO_PROFILES environment variable
// overrides the --profile flag, and that the test containers of a profile run after their dependencies
func TestOktetoTestProfilesFromEnvOverrideFlag(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithProfiledTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:     dir,
		Namespace:   testNamespace,
		OktetoHome:  dir,
		Token:       token,
		LogOutput:   "plain",
		NoCache:     true,
		Profiles:    []string{"pre-commit"},
		ProfilesEnv: "long-running",
	})
	require.NoError(t, err)

	require.Contains(t, out, "Using the profiles from OKTETO_PROFILES: long-running")
	require.Contains(t, out, "Test containers to run: seed, e2e")
	seed := strings.Index(out, "SEED-FROM-MANIFEST")
	require.NotEqual(t, -1, seed)
	require.Greater(t, strings.Index(out, "E2E-FROM-MANIFEST"), seed)
	require.NotContains(t, out, "UNIT-FROM-MANIFEST")
	require.NotContains(t, out, "LINT-FROM-MANIFEST")

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestProfilesFromDotEnv validates that OKTETO_PROFILES can be set in the .env file, which is
// loaded into the environment with the context
func TestOktetoTestProfilesFromDotEnv(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("OKTETO_PROFILES=release\n"), 0600))
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithProfiledTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
	})
	require.NoError(t, err)

	require.Contains(t, out, "LINT-FROM-MANIFEST")
	require.Contains(t, out, "SEED-FROM-MANIFEST")
	require.NotContains(t, out, "UNIT-FROM-MANIFEST")
	require.NotContains(t, out, "E2E-FROM-MANIFEST")

	require.ElementsMatch(t, []string{"lint", "seed"}, getInsightsTestNames(t, dir, testNamespace))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestProfileCannotBeCombinedWithTestNames validates that --profile cannot be combined
// with test container names. It fails before connecting to the cluster
func TestOktetoTestProfileCannotBeCombinedWithTestNames(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.yml"), []byte(oktetoManifestWithProfiledTests), 0600))

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		TestNames:  []string{"unit"},
		Profiles:   []string{"pre-commit"},
	})
	require.Error(t, err)
	require.Contains(t, out, "'--profile' cannot be used when naming the test containers to run")
	require.NotContains(t, out, "Executing test container")
}

// TestOktetoTestProfileCannotOverrideCommands validates that --profile cannot be combined with a
// command override. It fails before connecting to the cluster
func TestOktetoTestProfileCannotOverrideCommands(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.yml"), []byte(oktetoManifestWithProfiledTests), 0600))

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:     dir,
		OktetoHome:  dir,
		Token:       token,
		LogOutput:   "plain",
		Profiles:    []string{"pre-commit"},
		CommandArgs: []string{"echo", "override"},
	})
	require.Error(t, err)
	require.Contains(t, out, "'--profile' cannot be used when overriding the commands of a test container")
	require.NotContains(t, out, "Executing test container")
}
