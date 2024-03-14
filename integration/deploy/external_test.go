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
	b64 "encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

const (
	oktetoManifestWithExternalContent = `
deploy:
  - name: test command
    command: echo hola
external:
  test:
    notes: readme.md
    endpoints:
    - name: test-endpoint
      url: https://test.com
`
	notesContent = `## TEST
# This is a test
`
)

func Test_ExternalsFromOktetoManifestWithNotesContent(t *testing.T) {
	ctx := context.Background()
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createExternalNotes(dir))
	require.NoError(t, createManifest(dir))

	testNamespace := integration.GetTestNamespace("ExternalDeploy", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	_, cfg, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	externalControl := externalresource.NewExternalK8sControl(cfg)

	externals, err := externalControl.List(ctx, namespaceOpts.Namespace, "")

	require.NoError(t, err)
	require.Len(t, externals, 1)

	decodedContent, err := b64.StdEncoding.DecodeString(externals[0].Notes.Markdown)
	require.NoError(t, err)

	require.Equal(t, string(decodedContent), notesContent)

	// Check the endpoints command
	opts := &commands.EndpointOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Output:     "json",
	}

	output, err := commands.RunOktetoEndpoints(oktetoPath, opts)
	require.NoError(t, err)

	var endpoints []string
	err = json.Unmarshal([]byte(output), &endpoints)
	require.NoError(t, err)

	require.Greater(t, len(endpoints), 0)

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}

func createExternalNotes(dir string) error {
	notesPath := filepath.Join(dir, "readme.md")
	notesContent := []byte(notesContent)
	if err := os.WriteFile(notesPath, notesContent, 0600); err != nil {
		return err
	}
	return nil
}

func createManifest(dir string) error {
	manifestPath := filepath.Join(dir, "okteto.yml")
	manifestContent := []byte(oktetoManifestWithExternalContent)
	if err := os.WriteFile(manifestPath, manifestContent, 0600); err != nil {
		return err
	}
	return nil
}
