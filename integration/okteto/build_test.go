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
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const (
	manifestContentWithSecrets = `
build:
  test:
    context: .
    args:
    - RABBITMQ_PASS=${RABBITMQ_PASS}

deploy:
- echo "fake deploy"
`
	dockerfileUsingSecrets = `
FROM nginx
ARG RABBITMQ_PASS
RUN if [ -z "$RABBITMQ_PASS" ]; then exit 1; else echo $RABBITMQ_PASS; fi
`
)

func TestBuildReplaceSecretsInManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, createManifestV2(dir, manifestContentWithSecrets))
	require.NoError(t, createDockerfile(dir))
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)
	testNamespace := integration.GetTestNamespace("BuildWithSecrets", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	options := &commands.BuildOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}

	require.NoError(t, commands.RunOktetoBuild(oktetoPath, options))
}

func createDockerfile(dir string) error {
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfileContent := []byte(dockerfileUsingSecrets)
	return os.WriteFile(dockerfilePath, dockerfileContent, 0600)
}

func createManifestV2(dir, content string) error {
	manifestPath := filepath.Join(dir, "okteto.yml")
	manifestBytes := []byte(content)
	return os.WriteFile(manifestPath, manifestBytes, 0600)
}
