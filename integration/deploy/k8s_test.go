//go:build integration
// +build integration

// Copyright 2022 The Okteto Authors
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
	"github.com/stretchr/testify/require"
)

// TestDeployPipelineFromK8s tests the following scenario:
// - Deploying a pipeline manifest locally from a k8s file
// - The endpoints generated are accessible
func TestDeployPipelineFromK8s(t *testing.T) {
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createK8sManifest(dir))

	testNamespace := integration.GetTestNamespace("TestDeploy", user)
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, testNamespace))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, testNamespace)

	deployOptions := &commands.DeployOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	autowakeURL := fmt.Sprintf("https://e2etest-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, getContentFromURL(autowakeURL, timeout))

	destroyOptions := &commands.DestroyOptions{
		Workdir: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))
}
