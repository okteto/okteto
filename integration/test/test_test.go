//go:build integration
// +build integration

// Copyright 2024 The Okteto Authors
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
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oktetoManifestWithPassingTest = `
test:
  unit:
    context: .
    image: alpine
    commands:
      - echo "OK"

deploy:
  - echo "deploying"
`

	oktetoManifestWithPassingTestAndArtifacts = `
test:
  unit:
    context: .
    image: alpine
    commands:
      - echo "OK" > coverage.txt
      - mkdir -p reports && echo "OK" > reports/additional-coverage.txt
    artifacts:
      - coverage.txt
      - reports

deploy:
  - echo "deploying"
`

	oktetoManifestWithFailingTestAndArtifacts = `
test:
  unit:
    context: .
    image: alpine
    commands:
      - echo "NOT-OK" > coverage.txt
      - exit 1
    artifacts:
      - coverage.txt

deploy:
  - echo "deploying"
`
	dockerfileWithAnotherUser = `
FROM ubuntu:latest

WORKDIR /app
COPY . .

RUN chown -R 1000:1000 /app
USER 1000
`
	oktetoManifestWithImageReferencedInBuildSection = `
build:
  tests:
    context: .
test:
  hello:
    image: $OKTETO_BUILD_TESTS_IMAGE
    commands:
    - echo hello
`
)

var (
	token = ""
)

func TestMain(m *testing.M) {
	token = integration.GetToken()

	exitCode := m.Run()
	os.Exit(exitCode)
}

// TestOktetoTestsWithPassingTests validates the simplest happy path of okteto test
func TestOktetoTestsWithPassingTests(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	oktetoManifestPath := filepath.Join(dir, "okteto.yml")
	assert.NoError(t, os.WriteFile(oktetoManifestPath, []byte(oktetoManifestWithPassingTest), 0600))

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

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

// TestOktetoTestsWithPassingTestsAndArtifacts validates the happy path of okteto test with the export of artifacts
func TestOktetoTestsWithPassingTestsAndArtifacts(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	oktetoManifestPath := filepath.Join(dir, "okteto.yml")
	assert.NoError(t, os.WriteFile(oktetoManifestPath, []byte(oktetoManifestWithPassingTestAndArtifacts), 0600))

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

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
	coveragePath := filepath.Join(dir, "coverage.txt")
	coverage, err := os.ReadFile(coveragePath)
	require.NoError(t, err)
	assert.Equal(t, "OK\n", string(coverage))

	// check that 'reports' exists and is a directory
	reportsDirPath := filepath.Join(dir, "reports")
	reportsDir, err := os.Open(reportsDirPath)
	assert.NoError(t, err)
	reportsInfo, err := reportsDir.Stat()
	require.NoError(t, err)
	assert.True(t, reportsInfo.IsDir())

	// check that additional-coverage.txt exists and has the expected content
	additionalCoveragePath := filepath.Join(reportsDirPath, "additional-coverage.txt")
	additionalCoverage, err := os.ReadFile(additionalCoveragePath)
	require.NoError(t, err)
	assert.Equal(t, "OK\n", string(additionalCoverage))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestsWithFailingTestsAndArtifacts validates the scenario where tests fail but we are able to export artifacts anyway
func TestOktetoTestsWithFailingTestsAndArtifacts(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	oktetoManifestPath := filepath.Join(dir, "okteto.yml")
	assert.NoError(t, os.WriteFile(oktetoManifestPath, []byte(oktetoManifestWithFailingTestAndArtifacts), 0600))

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	testOptions := &commands.TestOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		NoCache:    true,
	}
	out, err := commands.RunOktetoTestAndGetOutput(oktetoPath, testOptions)
	require.Error(t, err)
	assert.Contains(t, out, "Test container 'unit' failed")
	coveragePath := filepath.Join(dir, "coverage.txt")
	coverage, err := os.ReadFile(coveragePath)
	require.NoError(t, err)
	assert.Equal(t, "NOT-OK\n", string(coverage))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

// TestOktetoTestsWithAnotherUser validates the simplest happy path of okteto test
func TestOktetoTestsWithAnotherUser(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	oktetoManifestPath := filepath.Join(dir, "okteto.yml")
	assert.NoError(t, os.WriteFile(oktetoManifestPath, []byte(oktetoManifestWithPassingTest), 0600))

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

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
