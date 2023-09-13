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
	"fmt"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

const (
	githubHTTPSURL = "https://github.com"
	pipelineRepo   = "okteto/movies"
	pipelineBranch = "cli-e2e"
)

func TestPipelineCommand(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace := integration.GetTestNamespace("TestPipeline", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	pipelineOptions := &commands.DeployPipelineOptions{
		Namespace:  testNamespace,
		Repository: fmt.Sprintf("%s/%s", githubHTTPSURL, pipelineRepo),
		Branch:     pipelineBranch,
		Wait:       true,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoDeployPipeline(oktetoPath, pipelineOptions))

	contentURL := fmt.Sprintf("https://movies-%s.%s", testNamespace, appsSubdomain)
	require.NotEmpty(t, integration.GetContentFromURL(contentURL, timeout))

	pipelineDestroyOptions := &commands.DestroyPipelineOptions{
		Namespace:  testNamespace,
		Name:       "movies",
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoPipelineDestroy(oktetoPath, pipelineDestroyOptions))
}
