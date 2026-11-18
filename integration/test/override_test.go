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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oktetoManifestWithDependentTests = `
test:
  lint:
    context: .
    image: alpine
    commands:
      - echo "LINT-FROM-MANIFEST"
  unit:
    context: .
    image: alpine
    depends_on:
      - lint
    commands:
      - echo "UNIT-FROM-MANIFEST"
`

	oktetoManifestWithSingleTest = `
test:
  unit:
    context: .
    image: alpine
    commands:
      - echo "UNIT-FROM-MANIFEST"
`

	oktetoManifestWithCachedTest = `
test:
  unit:
    context: .
    image: alpine
    skipIfNoFileChanges: true
    commands:
      - echo "UNIT-FROM-MANIFEST"
`

	// insightsLabelSelector selects the okteto insights events published by the CLI
	insightsLabelSelector = "events.okteto.com=true"
	// insightsTestAction is the action of the insights events published by okteto test
	insightsTestAction = "test"
)

// TestOktetoTestOverrideCommandsWithDash validates that the arguments after '--' replace the commands
// of the named test container, reaching the program unchanged, while its dependencies run with the commands
// defined in the manifest. The overridden run must not be published as an insights event
func TestOktetoTestOverrideCommandsWithDash(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithDependentTests)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:     dir,
		Namespace:   testNamespace,
		OktetoHome:  dir,
		Token:       token,
		LogOutput:   "plain",
		NoCache:     true,
		TestNames:   []string{"unit"},
		CommandArgs: []string{"printf", `[%s]\n`, "#smoke", "A|B", "it's", "$HOME"},
	})
	require.NoError(t, err)

	require.Contains(t, out, "LINT-FROM-MANIFEST")
	require.Contains(t, out, "Test container 'lint' passed")
	require.Contains(t, out, "Executing test container 'unit' with overridden commands")
	require.NotContains(t, out, "UNIT-FROM-MANIFEST")
	require.Contains(t, out, "[#smoke]")
	require.Contains(t, out, "[A|B]")
	require.Contains(t, out, "[it's]")
	require.Contains(t, out, "[$HOME]")
	require.Contains(t, out, "Test container 'unit' passed")

	require.ElementsMatch(t, []string{"lint"}, getInsightsTestNames(t, dir, testNamespace))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestOverrideCommandsWithCommandFlag validates that the --command flags replace the commands
// of the test container, running each one in order through the shell, and that a failing command fails the test
func TestOktetoTestOverrideCommandsWithCommandFlag(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithSingleTest)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
		TestNames:  []string{"unit"},
		Commands: []string{
			`printf 'step-%s\n' one && printf 'step-%s\n' two`,
			`printf 'step-%s\n' three`,
		},
	})
	require.NoError(t, err)

	require.NotContains(t, out, "UNIT-FROM-MANIFEST")
	stepOne := strings.Index(out, "step-one")
	stepTwo := strings.Index(out, "step-two")
	stepThree := strings.Index(out, "step-three")
	require.NotEqual(t, -1, stepOne)
	require.Greater(t, stepTwo, stepOne)
	require.Greater(t, stepThree, stepTwo)
	require.Contains(t, out, "Test container 'unit' passed")

	out, err = commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		NoCache:    true,
		TestNames:  []string{"unit"},
		Commands:   []string{"exit 1", `printf 'never-%s\n' reached`},
	})
	require.Error(t, err)
	require.Contains(t, out, "Test container 'unit' failed")
	require.NotContains(t, out, "never-reached")

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestOverrideCommandsIsNotServedFromTestCache validates that, for a test container with
// skipIfNoFileChanges, running it with overridden commands after a full run executes the overridden
// commands instead of reusing the cached result of the full run
func TestOktetoTestOverrideCommandsIsNotServedFromTestCache(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace, namespaceOpts := createTestNamespace(t, oktetoPath, dir, oktetoManifestWithCachedTest)

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		LogOutput:  "plain",
		TestNames:  []string{"unit"},
	})
	require.NoError(t, err)
	require.Contains(t, out, "UNIT-FROM-MANIFEST")

	out, err = commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:     dir,
		Namespace:   testNamespace,
		OktetoHome:  dir,
		Token:       token,
		LogOutput:   "plain",
		TestNames:   []string{"unit"},
		CommandArgs: []string{"printf", `override-%s\n`, "executed"},
	})
	require.NoError(t, err)
	require.Contains(t, out, "override-executed")
	require.NotContains(t, out, "UNIT-FROM-MANIFEST")

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestOverrideCommandsRequiresSingleTestName validates that overriding the commands
// fails when more than one test container is named. It fails before connecting to the cluster
func TestOktetoTestOverrideCommandsRequiresSingleTestName(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.yml"), []byte(oktetoManifestWithDependentTests), 0600))

	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, &commands.TestOptions{
		Workdir:     dir,
		OktetoHome:  dir,
		Token:       token,
		LogOutput:   "plain",
		TestNames:   []string{"lint", "unit"},
		CommandArgs: []string{"echo", "override"},
	})
	require.Error(t, err)
	require.Contains(t, out, "requires exactly one test container name, got 2")
	require.NotContains(t, out, "Executing test container")
}

// createTestNamespace writes the manifest in dir, creates a namespace for the test and writes
// its kubeconfig in dir. It returns the namespace name and the options to delete it
func createTestNamespace(t *testing.T, oktetoPath, dir, manifest string) (string, *commands.NamespaceOptions) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.yml"), []byte(manifest), 0600))

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
	return testNamespace, namespaceOpts
}

// getInsightsTestNames returns the test container names of the okteto test insights events published in the namespace
func getInsightsTestNames(t *testing.T, dir, namespace string) []string {
	t.Helper()
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	events, err := c.EventsV1().Events(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: insightsLabelSelector})
	require.NoError(t, err)

	names := []string{}
	for _, event := range events.Items {
		if event.Action != insightsTestAction {
			continue
		}
		var data struct {
			TestName string `json:"test_name"`
		}
		require.NoError(t, json.Unmarshal([]byte(event.Note), &data))
		names = append(names, data.TestName)
	}
	return names
}
