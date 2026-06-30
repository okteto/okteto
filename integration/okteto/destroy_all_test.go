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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// statusActive is the namespace status label value for an active namespace.
	// It is a bare literal because the CLI does not expose a constant for it.
	statusActive = "Active"

	destroyAllDevEnvName = "destroyall"

	destroyAllK8sManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-destroy-all
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-destroy-all
  template:
    metadata:
      labels:
        app: e2e-destroy-all
    spec:
      terminationGracePeriodSeconds: 1
      containers:
      - name: test
        image: nginx:alpine
`

	destroyAllOktetoManifest = `deploy:
- kubectl apply -f k8s.yml
`
)

// TestDestroyAllPreservesNamespaceStatus verifies that `okteto destroy --all`:
//   - completes and leaves the namespace "Active" when it was active before the command, and
//   - completes and keeps the namespace "Sleeping" when it was sleeping before the command.
//
// The sleeping case is a regression test: the backend now restores the namespace to its
// previous status after destroy all (Active or Sleeping). Previously the CLI wait loop only
// understood "Active", so destroying a sleeping namespace hung until the 5-minute timeout.
func TestDestroyAllPreservesNamespaceStatus(t *testing.T) {
	integration.SkipIfNotOktetoCluster(t)
	t.Parallel()

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, createDestroyAllManifests(dir))

	testNamespace := integration.GetTestNamespace(t.Name())
	namespaceOpts := &commands.NamespaceOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
	}
	require.NoError(t, commands.RunOktetoCreateNamespace(oktetoPath, namespaceOpts))
	t.Cleanup(func() {
		require.NoError(t, commands.RunOktetoDeleteNamespace(oktetoPath, namespaceOpts))
	})
	require.NoError(t, commands.RunOktetoKubeconfig(oktetoPath, &commands.KubeconfigOpts{
		OktetoHome: dir,
	}))

	c, _, err := okteto.NewK8sClientProvider().Provide(kubeconfig.Get([]string{filepath.Join(dir, ".kube", "config")}))
	require.NoError(t, err)

	ctx := context.Background()
	devEnvConfigMap := fmt.Sprintf("okteto-git-%s", destroyAllDevEnvName)

	deployOptions := &commands.DeployOptions{
		Workdir:    dir,
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		Name:       destroyAllDevEnvName,
	}
	destroyAllOptions := &commands.DestroyOptions{
		Namespace:  testNamespace,
		OktetoHome: dir,
		Token:      token,
		All:        true,
	}

	// 1) Active namespace: deploy a dev environment, then destroy all.
	// The namespace must end up clean and remain Active.
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	_, err = integration.GetConfigmap(ctx, testNamespace, devEnvConfigMap, c)
	require.NoError(t, err)

	require.NoError(t, commands.RunOktetoDestroyAll(oktetoPath, destroyAllOptions))

	_, err = integration.GetConfigmap(ctx, testNamespace, devEnvConfigMap, c)
	require.True(t, k8sErrors.IsNotFound(err), "dev environment configmap should be destroyed")
	ns, err := integration.GetNamespace(ctx, testNamespace, c)
	require.NoError(t, err)
	require.Equal(t, statusActive, ns.Labels[constants.NamespaceStatusLabel], "namespace should remain Active")

	// 2) Sleeping namespace: deploy again, put the namespace to sleep, then destroy all.
	// The namespace must end up clean and remain Sleeping (the regression scenario).
	require.NoError(t, commands.RunOktetoDeploy(oktetoPath, deployOptions))
	_, err = integration.GetConfigmap(ctx, testNamespace, devEnvConfigMap, c)
	require.NoError(t, err)

	require.NoError(t, commands.RunOktetoNamespaceSleep(oktetoPath, namespaceOpts))
	require.NoError(t, integration.WaitForNamespaceStatus(ctx, testNamespace, constants.NamespaceStatusSleeping, c, 2*time.Minute))

	require.NoError(t, commands.RunOktetoDestroyAll(oktetoPath, destroyAllOptions))

	_, err = integration.GetConfigmap(ctx, testNamespace, devEnvConfigMap, c)
	require.True(t, k8sErrors.IsNotFound(err), "dev environment configmap should be destroyed")
	ns, err = integration.GetNamespace(ctx, testNamespace, c)
	require.NoError(t, err)
	require.Equal(t, constants.NamespaceStatusSleeping, ns.Labels[constants.NamespaceStatusLabel], "namespace should remain Sleeping")
}

func createDestroyAllManifests(dir string) error {
	if err := os.WriteFile(filepath.Join(dir, "k8s.yml"), []byte(destroyAllK8sManifest), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "okteto.yml"), []byte(destroyAllOktetoManifest), 0600); err != nil {
		return err
	}
	return nil
}
