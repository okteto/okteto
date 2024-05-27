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

package up

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const (
	apiDockerfile = `FROM alpine`

	oktetoManifest = `build:
  api:
    context: api

deploy:
  compose: docker-compose.yml
`

	composeTemplateWithoutBuild = `services:
  api:
    image: ${OKTETO_BUILD_API_IMAGE}
    command: /bin/sh
    workdir: /usr/src
    volumes:
    - .:/usr/src
`
)

// TestUpComposeWithOktetoManifest validates the scenario where the docker-compose defines a service, but it relies
// on the okteto manifest to build the image. This test ensures commands like 'down' and 'destroy' that do not build the
// service, work as expected.
func TestUpComposeWithOktetoManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testNamespace := integration.GetTestNamespace("UpComposeAndManifest", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))

	require.NoError(t, os.Mkdir(filepath.Join(dir, "api"), 0700))
	require.NoError(t, writeFile(filepath.Join(dir, "api", "Dockerfile"), apiDockerfile))
	require.NoError(t, writeFile(filepath.Join(dir, "okteto.yml"), oktetoManifest))
	require.NoError(t, writeFile(filepath.Join(dir, "docker-compose.yml"), composeTemplateWithoutBuild))
	require.NoError(t, writeFile(filepath.Join(dir, ".stignore"), stignoreContent))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	upOptions := &commands.UpOptions{
		Name:       "api",
		Namespace:  testNamespace,
		Workdir:    dir,
		OktetoHome: dir,
		Token:      token,
	}
	upResult, err := commands.RunOktetoUp(oktetoPath, upOptions)
	require.NoError(t, err)

	// Test okteto down command
	downOpts := &commands.DownOptions{
		Namespace: testNamespace,
		Workdir:   dir,
		Token:     token,
	}
	require.NoError(t, commands.RunOktetoDown(oktetoPath, downOpts))

	require.True(t, commands.HasUpCommandFinished(upResult.Pid.Pid))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
