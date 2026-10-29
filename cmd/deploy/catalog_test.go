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

package deploy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/catalog"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// catalogManifest returns a manifest with deploy commands that run remotely when remote is true
func catalogManifest(remote bool) *model.Manifest {
	return &model.Manifest{
		Deploy: &model.DeployInfo{
			Commands: []model.DeployCommand{{Name: "echo", Command: "echo"}},
			Remote:   &remote,
		},
	}
}

// newCatalogRunCommand returns a Command that deploys the manifest provided, getting the names of the secret
// variables of the catalog item from secrets
func newCatalogRunCommand(t *testing.T, manifest *model.Manifest, deployer *fakeDeployer, secrets *fakeCatalogSecretVariables, objects ...runtime.Object) *Command {
	t.Helper()
	resetLogBuffer(t)
	t.Setenv(constants.OktetoForceRemote, "")
	setTestOktetoContext()
	k8sProvider := test.NewFakeK8sProvider(objects...)
	return &Command{
		AnalyticsTracker:          &fakeTracker{},
		GetManifest:               func(string, afero.Fs) (*model.Manifest, error) { return manifest, nil },
		K8sClientProvider:         k8sProvider,
		EndpointGetter:            getFakeEndpoint,
		Fs:                        afero.NewMemMapFs(),
		CfgMapHandler:             newDefaultConfigMapHandler(k8sProvider, nil),
		GetDeployer:               deployer.Get,
		Builder:                   &fakeV2Builder{},
		IoCtrl:                    io.NewIOController(),
		GetCatalogSecretVariables: secrets.get,
	}
}

// expectDeploy makes the deployer succeed when the deploy forwards the catalog item provided to remote-run
func expectDeploy(deployer *fakeDeployer, forwarded catalog.Item) {
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.MatchedBy(func(opts *Options) bool {
		return opts.catalogItemToForward == forwarded
	})).Return(nil)
}

func TestRunForwardsTheCatalogItemWhenItsSecretVariablesRunRemotely(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	deployer := &fakeDeployer{}
	expectDeploy(deployer, testCatalogItem)
	c := newCatalogRunCommand(t, catalogManifest(true), deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.NoError(t, err)
	require.Equal(t, []string{"item-id"}, secrets.ids)
	deployer.AssertExpectations(t)
}

func TestRunFailsBeforeAnyCommandWhenCatalogSecretVariablesWouldRunLocally(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	// no expectations: no command must run
	deployer := &fakeDeployer{}
	c := newCatalogRunCommand(t, catalogManifest(false), deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.ErrorIs(t, err, catalog.ErrSecretVariablesNeedRemote)
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertNotCalled(t, "Get")
}

func TestRunFailsBeforeAnyCommandWhenCatalogSecretVariablesCannotBeChecked(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{err: assert.AnError}
	deployer := &fakeDeployer{}
	c := newCatalogRunCommand(t, catalogManifest(true), deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.ErrorIs(t, err, assert.AnError)
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertNotCalled(t, "Get")
}

func TestRunDeploysADeletedCatalogItemWithoutForwardingIt(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{err: oktetoErrors.ErrCatalogItemNotFound}
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.Item{})
	c := newCatalogRunCommand(t, catalogManifest(true), deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.NoError(t, err)
	deployer.AssertExpectations(t)
}

func TestRunDeploysLocallyACatalogItemWithoutSecretVariables(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{}
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.Item{})
	c := newCatalogRunCommand(t, catalogManifest(false), deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.NoError(t, err)
	require.Equal(t, []string{"item-id"}, secrets.ids)
	deployer.AssertExpectations(t)
}

func TestRunWithoutCatalogItemDoesNotCheckSecretVariables(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.Item{})
	c := newCatalogRunCommand(t, catalogManifest(false), deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test"})

	require.NoError(t, err)
	require.Empty(t, secrets.ids)
	deployer.AssertExpectations(t)
}

func TestRunWithoutDeployCommandsDoesNotCheckCatalogSecretVariables(t *testing.T) {
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.Item{})
	c := newCatalogRunCommand(t, &model.Manifest{Deploy: &model.DeployInfo{}}, deployer, secrets)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.NoError(t, err)
	require.Empty(t, secrets.ids)
	deployer.AssertExpectations(t)
}

func TestReadCatalogItemAndSetVariablesReadsTheItemBeforeAVariableWithItsName(t *testing.T) {
	t.Setenv(catalog.ItemIDEnvVar, testCatalogItem.ID)
	t.Setenv(catalog.SecretVariablesEndpointEnvVar, testCatalogItem.SecretVariablesEndpoint)
	options := &Options{Variables: []string{
		"OKTETO_CATALOG_ITEM_ID=var-id",
		"OKTETO_CATALOG_SECRET_VARIABLES_ENDPOINT=https://var.example",
	}}

	err := readCatalogItemAndSetVariables(options, setTestEnv(t))

	require.NoError(t, err)
	require.Equal(t, testCatalogItem, options.CatalogItem)
	require.Equal(t, "var-id", os.Getenv(catalog.ItemIDEnvVar))
}

func TestReadCatalogItemAndSetVariablesIgnoresAVariableWithItsNameOutsideTheInstaller(t *testing.T) {
	t.Setenv(catalog.ItemIDEnvVar, "")
	t.Setenv(catalog.SecretVariablesEndpointEnvVar, "")
	options := &Options{Variables: []string{"OKTETO_CATALOG_ITEM_ID=var-id"}}

	err := readCatalogItemAndSetVariables(options, setTestEnv(t))

	require.NoError(t, err)
	require.Equal(t, catalog.Item{}, options.CatalogItem)
}

// setTestEnv sets env vars that are restored when the test ends
func setTestEnv(t *testing.T) func(key, value string) error {
	t.Helper()
	return func(key, value string) error {
		t.Setenv(key, value)
		return nil
	}
}

func TestRunInInstallerKeepsTheCatalogItemWhenAStoredVariableHasItsName(t *testing.T) {
	stored, err := json.Marshal([]types.DeployVariable{{Name: catalog.ItemIDEnvVar, Value: "stored-id"}})
	require.NoError(t, err)
	cfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName("movies"),
			Namespace: "test",
			Labels:    map[string]string{model.GitDeployLabel: "true"},
		},
		Data: map[string]string{"variables": base64.StdEncoding.EncodeToString(stored)},
	}
	secrets := &fakeCatalogSecretVariables{names: []string{"API_TOKEN"}}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.MatchedBy(func(opts *Options) bool {
		return opts.catalogItemToForward == testCatalogItem && opts.Variables[0] == "OKTETO_CATALOG_ITEM_ID=stored-id"
	})).Return(nil)
	c := newCatalogRunCommand(t, catalogManifest(true), deployer, secrets, cfg)
	c.RunningInInstaller = true

	err = c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogItem: testCatalogItem})

	require.NoError(t, err)
	require.Equal(t, []string{"item-id"}, secrets.ids)
	deployer.AssertExpectations(t)
}
