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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	oktetoManifestWithCustomImage = `build:
    app:
      context: app
      image: okteto.dev/test-app:1.0.0
deploy:
- kubectl apply -f k8s.yml
`
)

// TestDeployWithSmartBuildCloneCustomImage tests the following scenario:
// - Build in another namespace to generate image in the global registry
// - Deploy an application with a custom image
// - Verify the image is not built and used the one previously built
// - Check that the deployment is successful and the image is the one expected
func TestDeployWithSmartBuildCloneCustomImage(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()

	globalBuildNamespace := integration.GetTestNamespace(t.Name())
	globalNamespaceOpts := &commands.NamespaceOptions{
		Namespace:  globalBuildNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, globalNamespaceOpts))

	require.NoError(t, createAppDockerfile(dir))
	require.NoError(t, createOktetoManifestWithCustomImage(dir))
	require.NoError(t, integration.GitInit(dir))

	testNamespace := integration.GetTestNamespace(t.Name())
	require.NoError(t, createK8sManifestWithCache(dir, fmt.Sprintf("%s/%s/test-app:1.0.0", okteto.GetContext().Registry, testNamespace)))

	buildOptions := &commands.BuildOptions{
		Workdir:    dir,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoBuild(oktetoPath, buildOptions))

	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, &commands.KubeconfigOpts{
		OktetoHome: dir,
	}))
	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Wait:       true,
	}

	outpput, err := commands.RunOktetoDeployAndGetOutput(oktetoPath, deployOptions)
	require.NoError(t, err)
	require.Contains(t, outpput, "Okteto Smart Builds is skipping build of 'app' because it's already built from cache.")

	// Test that image has been built
	require.NotEmpty(t, getImageWithSHA(fmt.Sprintf("%s/%s/test-app:1.0.0", okteto.GetContext().Registry, testNamespace)))

	destroyOptions := &commands.DestroyOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
	}
	require.NoError(t, commands.RunOktetoDestroy(oktetoPath, destroyOptions))

	_, err = integration.GetService(context.Background(), testNamespace, "e2etest", c)
	require.True(t, k8sErrors.IsNotFound(err))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, globalNamespaceOpts))
	require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
}

func createOktetoManifestWithCustomImage(dir string) error {
	dockerfilePath := filepath.Join(dir, oktetoManifestName)
	dockerfileContent := []byte(oktetoManifestWithCustomImage)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0600); err != nil {
		return err
	}
	return nil
}
