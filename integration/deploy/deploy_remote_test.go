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

package deploy

import (
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

var (
	parentManifestContent = `
deploy:
  remote: true
  commands:
    - okteto deploy -f other-okteto.yml --remote`

	childManifestContent = `
deploy:
  image: aquasec/trivy:latest
  commands:
    - trivy -q image --light alpine:3.14`
)

// TestDeployInDeployRemote test the scenario where an okteto deploy is run inside an okteto deploy in remote
// image base for the child deploy should be the specified at the child manifest and not the parent
func TestDeployInDeployRemote(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	require.NoError(t, createOktetoManifestWithName(dir, parentManifestContent, "okteto.yml"))
	require.NoError(t, createOktetoManifestWithName(dir, childManifestContent, "other-okteto.yml"))

	testNamespace := integration.GetTestNamespace("DeployInDeployRemote", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		Token:      token,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}
