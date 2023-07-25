//go:build e2e
// +build e2e

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
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	"github.com/okteto/okteto/e2e"
	"github.com/okteto/okteto/e2e/commands"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const (
	hybridManifest = `
deploy:
  compose: docker-compose.yml
dev:
  svc:
    context: svc
    namespace: user
    mode: hybrid
    command: "{{ .Shell }} checker.sh"
    reverse:
    - 8080:8080
`
	hybridCompose = `services:
 svc:
  build: .
  environment:
  - ENV_IN_POD=value_from_pod`

	svcDockerfile = `FROM busybox
ENV ENV_IN_IMAGE value_from_image`
	localProcess = `
#!/bin/bash

for x in ENV_IN_POD,value_from_pod ENV_IN_IMAGE,value_from_image ; do
  IFS=, read name value <<< "$x"
  if [ "${!name}" != "$value" ]; then
    echo "env '$name' not found. Expected value '$value'"
    exit 1
  fi
done 

echo "!Successful envs check!"
exit 0`
)

// TestUpUsingHybridMode test hybrid mode checking:
// - envs from config map, image and dev container are available
// - we cannot run a micro application locally and check that it is exposed
// using the reverse in the remote because we do not know the dependencies
// locally. The testing of the reverse is addressed in other tests
func TestUpUsingHybridMode(t *testing.T) {
	t.Parallel()
	// Prepare environment
	dir := t.TempDir()
	oktetoPath, err := e2e.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := e2e.GetTestNamespace("TestHybridMode", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	tmpl := template.Must(template.New("okteto.yml").Parse(hybridManifest))

	oktetoManifestFileName := filepath.Join(dir, "okteto.yml")
	fs := afero.NewOsFs()
	oktetoManifestFile, err := fs.Create(oktetoManifestFileName)
	require.NoError(t, err)

	type oktetoManifestTemplate struct {
		Shell string
	}

	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "sh"
	}
	oktetoManifestSyntax := oktetoManifestTemplate{
		Shell: shell,
	}

	require.NoError(t, tmpl.Execute(oktetoManifestFile, oktetoManifestSyntax))
	require.NoError(t, writeFile(filepath.Join(dir, "docker-compose.yml"), hybridCompose))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))
	require.NoError(t, writeFile(filepath.Join(dir, "Dockerfile"), svcDockerfile))
	require.NoError(t, writeFile(filepath.Join(dir, "checker.sh"), localProcess))

	up1Options := &commands.UpOptions{
		Name:       "svc",
		Namespace:  testNamespace,
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
		Service:    "svc",
		Deploy:     true,
	}
	output, err := commands.RunOktetoUpAndWaitWithOutput(oktetoPath, up1Options)
	require.NoError(t, err)

	// Test warnings for unsupported fields
	require.Contains(t, output.String(), "In hybrid mode, the field(s) 'context, namespace' specified in your manifest are ignored")

	// Test okteto down command
	down1Opts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Service:   "svc",
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, down1Opts))
}
