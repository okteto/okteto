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
	"fmt"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

var (
	deployRemoteWithBuildCommandManifestContent = `
deploy:
  remote: true
  commands:
    - okteto build -t okteto.dev/testing-image:test -f app/Dockerfile --no-cache`
)

// TestDeployRemoteWithBuildCommand tests the following scenario:
// - Deploying a okteto manifest in remote with a build command
// - Check that is not running on depot
func TestDeployRemoteWithBuildCommand(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	testNamespace := integration.GetTestNamespace("DeployRemoteBuild", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	require.NoError(t, createOktetoManifest(dir, deployRemoteWithBuildCommandManifestContent))
	require.NoError(t, createAppDockerfile(dir))

	t.Setenv(build.DepotTokenEnvVar, "fakeToken")
	t.Setenv(build.DepotProjectEnvVar, "fakeProject")
	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	// Test that image has been built
	require.NotEmpty(t, getImageWithSHA(fmt.Sprintf("%s/%s/testing-image:test", okteto.GetContext().Registry, testNamespace)))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroyRemote(oktetoPath, destroyOptions))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
