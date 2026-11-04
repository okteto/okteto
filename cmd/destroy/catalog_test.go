// Copyright 2026 The Okteto Authors
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

package destroy

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/catalog"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	okerrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
)

var testCatalogItem = catalog.Item{
	ID:                      "item-id",
	SecretVariablesEndpoint: "https://okteto.example/internal/catalog/secret-variables",
}

type fakeCatalogSecretVariables struct {
	err   error
	names []string
	ids   []string
}

func (f *fakeCatalogSecretVariables) get(_ context.Context, id string) ([]string, error) {
	f.ids = append(f.ids, id)
	return f.names, f.err
}

// catalogManifest returns a manifest with destroy commands that run remotely when remote is true
func catalogManifest(remote bool) *model.Manifest {
	return &model.Manifest{
		Name: "test-app",
		Destroy: &model.DestroyInfo{
			Commands: []model.DeployCommand{{Name: "echo", Command: "echo"}},
			Remote:   &remote,
		},
	}
}

// catalogDestroy is a destroy command that runs its commands locally with executor and gets the names of the
// secret variables of the catalog item from secrets
type catalogDestroy struct {
	cmd       *destroyCommand
	client    kubernetes.Interface
	destroyer *fakeDestroyer
	executor  *fakeExecutor
}

func newCatalogDestroy(t *testing.T, secrets *fakeCatalogSecretVariables) *catalogDestroy {
	t.Helper()
	t.Setenv(constants.OktetoForceRemote, "")
	t.Setenv(constants.OktetoDeployRemote, "")
	k8sClientProvider := test.NewFakeK8sProvider()
	client, _, err := k8sClientProvider.Provide(api.NewConfig())
	require.NoError(t, err)
	destroyer := &fakeDestroyer{}
	executor := &fakeExecutor{}
	return &catalogDestroy{
		cmd: &destroyCommand{
			ConfigMapHandler:  NewConfigmapHandler(client),
			nsDestroyer:       destroyer,
			secrets:           &fakeSecretHandler{},
			k8sClientProvider: k8sClientProvider,
			executor:          executor,
			buildCtrlProvider: fakeBuildCtrlProvider{
				buildCtrl: buildCtrl{
					builder: fakeBuilderV2{getSvcs: fakeGetSvcs{}},
				},
			},
			getCatalogSecretVariables: secrets.get,
		},
		client:    client,
		destroyer: destroyer,
		executor:  executor,
	}
}

func (cd *catalogDestroy) requireStatus(t *testing.T, expected string) {
	t.Helper()
	cfg, err := cd.client.CoreV1().ConfigMaps("namespace").Get(context.Background(), pipeline.TranslatePipelineName("test-app"), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, expected, cfg.Data["status"])
}

func TestDestroyFailsBeforeAnyCommandWhenCatalogSecretVariablesWouldRunLocally(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	cd := newCatalogDestroy(t, secrets)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:        "test-app",
		Namespace:   "namespace",
		Manifest:    catalogManifest(false),
		CatalogItem: testCatalogItem,
	})

	require.ErrorIs(t, err, catalog.ErrSecretVariablesNeedRemote)
	require.Empty(t, cd.executor.executed)
	require.False(t, cd.destroyer.destroyed)
	cd.requireStatus(t, pipeline.ErrorStatus)
}

func TestDestroyFailsBeforeAnyCommandWhenTheCatalogItemWasDeleted(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{err: okerrors.ErrCatalogItemNotFound}
	cd := newCatalogDestroy(t, secrets)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:        "test-app",
		Namespace:   "namespace",
		Manifest:    catalogManifest(true),
		CatalogItem: testCatalogItem,
	})

	require.ErrorContains(t, err, "catalog item 'item-id' not found")
	require.Empty(t, cd.executor.executed)
	require.False(t, cd.destroyer.destroyed)
	cd.requireStatus(t, pipeline.ErrorStatus)
}

func TestDestroyFailsBeforeAnyCommandWhenCatalogSecretVariablesCannotBeChecked(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{err: assert.AnError}
	cd := newCatalogDestroy(t, secrets)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:        "test-app",
		Namespace:   "namespace",
		Manifest:    catalogManifest(true),
		CatalogItem: testCatalogItem,
	})

	require.ErrorIs(t, err, assert.AnError)
	require.Empty(t, cd.executor.executed)
	require.False(t, cd.destroyer.destroyed)
	cd.requireStatus(t, pipeline.ErrorStatus)
}

func TestForceDestroyForwardsTheCatalogItemWhenTheCheckPasses(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	cd := newCatalogDestroy(t, secrets)
	opts := &Options{
		Name:         "test-app",
		Manifest:     catalogManifest(true),
		CatalogItem:  testCatalogItem,
		ForceDestroy: true,
	}

	err := cd.cmd.checkCatalogSecretVariables(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, testCatalogItem, opts.catalogItemToForward)
}

func TestForceDestroyRunsTheCommandsWithoutTheCatalogSecretVariablesWhenTheCheckFails(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{err: assert.AnError}
	cd := newCatalogDestroy(t, secrets)
	opts := &Options{
		Name:         "test-app",
		Namespace:    "namespace",
		Manifest:     catalogManifest(false),
		CatalogItem:  testCatalogItem,
		ForceDestroy: true,
	}

	err := cd.cmd.destroy(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, []string{"item-id"}, secrets.ids)
	require.Equal(t, catalog.Item{}, opts.catalogItemToForward)
	require.Len(t, cd.executor.executed, 1)
	require.True(t, cd.destroyer.destroyed)
}

func TestDestroyLocallyACatalogItemWithoutSecretVariables(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{}
	cd := newCatalogDestroy(t, secrets)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:        "test-app",
		Namespace:   "namespace",
		Manifest:    catalogManifest(false),
		CatalogItem: testCatalogItem,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"item-id"}, secrets.ids)
	require.Len(t, cd.executor.executed, 1)
	require.True(t, cd.destroyer.destroyed)
}

func TestDestroyWithoutCatalogItemDoesNotCheckSecretVariables(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	cd := newCatalogDestroy(t, secrets)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:      "test-app",
		Namespace: "namespace",
		Manifest:  catalogManifest(false),
	})

	require.NoError(t, err)
	require.Empty(t, secrets.ids)
	require.Len(t, cd.executor.executed, 1)
}

func TestDestroyWithoutDestroyCommandsDoesNotCheckCatalogSecretVariables(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{err: okerrors.ErrCatalogItemNotFound}
	cd := newCatalogDestroy(t, secrets)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:        "test-app",
		Namespace:   "namespace",
		Manifest:    &model.Manifest{Name: "test-app", Destroy: &model.DestroyInfo{}},
		CatalogItem: testCatalogItem,
	})

	require.NoError(t, err)
	require.Empty(t, secrets.ids)
	require.True(t, cd.destroyer.destroyed)
}

func TestCheckCatalogSecretVariablesForwardsTheItemWhenDestroyRunsRemotely(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	cd := newCatalogDestroy(t, secrets)
	opts := &Options{
		Name:        "test-app",
		Manifest:    catalogManifest(true),
		CatalogItem: testCatalogItem,
	}

	err := cd.cmd.checkCatalogSecretVariables(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, testCatalogItem, opts.catalogItemToForward)
}
