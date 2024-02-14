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
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
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
	testNamespace := integration.GetTestNamespace("Pipeline", user)
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

func TestPipelineDeployWithReuse(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	testNamespace := integration.GetTestNamespace("PipelineDeployWithReuse", user)
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	defer commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts)

	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, dir))
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	pipelineOptions := &commands.DeployPipelineOptions{
		Namespace:  testNamespace,
		Repository: "https://github.com/okteto/go-getting-started",
		Branch:     "cli-e2e",
		Wait:       true,
		OktetoHome: dir,
		Token:      token,
		Name:       "test-pipeline",
		Labels:     []string{"test-label"},
	}
	require.NoError(t, commands.RunOktetoDeployPipeline(oktetoPath, pipelineOptions))

	cmap, err := integration.GetConfigmap(context.Background(), testNamespace, "okteto-git-test-pipeline", c)
	require.NoError(t, err)

	require.Contains(t, cmap.Labels, "label.okteto.com/test-label")

	pipelineOptionsWithReuse := &commands.DeployPipelineOptions{
		Namespace:   testNamespace,
		Repository:  "https://github.com/okteto/go-getting-started",
		Branch:      "ignored-value",
		Wait:        true,
		OktetoHome:  dir,
		Token:       token,
		Name:        "test-pipeline",
		ReuseParams: true,
	}

	require.NoError(t, commands.RunOktetoDeployPipeline(oktetoPath, pipelineOptionsWithReuse))

	cmapRedeploy, err := integration.GetConfigmap(context.Background(), testNamespace, "okteto-git-test-pipeline", c)
	require.NoError(t, err)

	require.Contains(t, cmapRedeploy.Labels, "label.okteto.com/test-label")
	require.Equal(t, cmap.CreationTimestamp, cmapRedeploy.CreationTimestamp)

}
