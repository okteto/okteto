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
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var testCatalogSecretVariables = catalog.SecretVariables{
	References: `[{"name":"API_TOKEN","secretKeyRef":{"name":"creds","key":"token"}}]`,
	Action:     "action-name",
	Endpoint:   "https://okteto.example/internal/catalog/secret-variables",
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

// newCatalogRunCommand returns a Command that deploys the manifest provided
func newCatalogRunCommand(t *testing.T, manifest *model.Manifest, deployer *fakeDeployer, objects ...runtime.Object) *Command {
	t.Helper()
	resetLogBuffer(t)
	t.Setenv(constants.OktetoForceRemote, "")
	setTestOktetoContext()
	k8sProvider := test.NewFakeK8sProvider(objects...)
	return &Command{
		AnalyticsTracker:  &fakeTracker{},
		GetManifest:       func(string, afero.Fs) (*model.Manifest, error) { return manifest, nil },
		K8sClientProvider: k8sProvider,
		EndpointGetter:    getFakeEndpoint,
		Fs:                afero.NewMemMapFs(),
		CfgMapHandler:     newDefaultConfigMapHandler(k8sProvider, nil),
		GetDeployer:       deployer.Get,
		Builder:           &fakeV2Builder{},
		IoCtrl:            io.NewIOController(),
	}
}

// expectDeploy makes the deployer succeed when the deploy forwards the catalog secret variables provided to
// remote-run
func expectDeploy(deployer *fakeDeployer, forwarded catalog.SecretVariables) {
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.MatchedBy(func(opts *Options) bool {
		return opts.catalogSecretVariablesToForward == forwarded
	})).Return(nil)
}

func TestRunForwardsTheCatalogSecretVariablesWhenTheCommandsRunRemotely(t *testing.T) {
	deployer := &fakeDeployer{}
	expectDeploy(deployer, testCatalogSecretVariables)
	c := newCatalogRunCommand(t, catalogManifest(true), deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: testCatalogSecretVariables})

	require.NoError(t, err)
	deployer.AssertExpectations(t)
}

func TestRunFailsBeforeAnyCommandWhenCatalogSecretVariablesWouldRunLocally(t *testing.T) {
	// no expectations: no command must run
	deployer := &fakeDeployer{}
	c := newCatalogRunCommand(t, catalogManifest(false), deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: testCatalogSecretVariables})

	require.ErrorIs(t, err, catalog.ErrSecretVariablesNeedRemote)
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertNotCalled(t, "Get")
}

func TestRunFailsBeforeAnyCommandWhenCatalogSecretVariablesCannotBeRead(t *testing.T) {
	deployer := &fakeDeployer{}
	c := newCatalogRunCommand(t, catalogManifest(true), deployer)
	secretVariables := testCatalogSecretVariables
	secretVariables.References = `{"name":`

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: secretVariables})

	require.ErrorContains(t, err, "could not read the catalog secret variables")
	requireDeployStatus(t, c, pipeline.ErrorStatus)
	deployer.AssertNotCalled(t, "Get")
}

func TestRunDeploysLocallyWithoutCatalogSecretVariables(t *testing.T) {
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.SecretVariables{})
	c := newCatalogRunCommand(t, catalogManifest(false), deployer)
	secretVariables := testCatalogSecretVariables
	secretVariables.References = "[]"

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: secretVariables})

	require.NoError(t, err)
	deployer.AssertExpectations(t)
}

func TestRunWithoutCatalogSecretVariablesDoesNotForwardThem(t *testing.T) {
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.SecretVariables{})
	c := newCatalogRunCommand(t, catalogManifest(false), deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: catalog.SecretVariables{Action: "action-name"}})

	require.NoError(t, err)
	deployer.AssertExpectations(t)
}

func TestRunWithoutDeployCommandsDoesNotCheckCatalogSecretVariables(t *testing.T) {
	deployer := &fakeDeployer{}
	expectDeploy(deployer, catalog.SecretVariables{})
	c := newCatalogRunCommand(t, &model.Manifest{Deploy: &model.DeployInfo{}}, deployer)

	err := c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: testCatalogSecretVariables})

	require.NoError(t, err)
	deployer.AssertExpectations(t)
}

func TestReadCatalogSecretVariablesAndSetVariablesReadsThemBeforeAVariableWithTheirName(t *testing.T) {
	t.Setenv(catalog.SecretVariablesEnvVar, testCatalogSecretVariables.References)
	t.Setenv(model.OktetoActionNameEnvVar, testCatalogSecretVariables.Action)
	t.Setenv(catalog.SecretVariablesEndpointEnvVar, testCatalogSecretVariables.Endpoint)
	options := &Options{Variables: []string{
		"OKTETO_CATALOG_SECRET_VARIABLES=[]",
		"OKTETO_CATALOG_SECRET_VARIABLES_ENDPOINT=https://var.example",
	}}

	err := readCatalogSecretVariablesAndSetVariables(options, setTestEnv(t))

	require.NoError(t, err)
	require.Equal(t, testCatalogSecretVariables, options.CatalogSecretVariables)
	require.Equal(t, "[]", os.Getenv(catalog.SecretVariablesEnvVar))
}

func TestReadCatalogSecretVariablesAndSetVariablesIgnoresAVariableWithTheirNameOutsideTheInstaller(t *testing.T) {
	t.Setenv(catalog.SecretVariablesEnvVar, "")
	t.Setenv(model.OktetoActionNameEnvVar, "")
	t.Setenv(catalog.SecretVariablesEndpointEnvVar, "")
	options := &Options{Variables: []string{`OKTETO_CATALOG_SECRET_VARIABLES=[{"name":"API_TOKEN"}]`}}

	err := readCatalogSecretVariablesAndSetVariables(options, setTestEnv(t))

	require.NoError(t, err)
	require.Equal(t, catalog.SecretVariables{}, options.CatalogSecretVariables)
}

// setTestEnv sets env vars that are restored when the test ends
func setTestEnv(t *testing.T) func(key, value string) error {
	t.Helper()
	return func(key, value string) error {
		t.Setenv(key, value)
		return nil
	}
}

func TestRunInInstallerKeepsTheCatalogSecretVariablesWhenAStoredVariableHasTheirName(t *testing.T) {
	stored, err := json.Marshal([]types.DeployVariable{{Name: catalog.SecretVariablesEnvVar, Value: "[]"}})
	require.NoError(t, err)
	cfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName("movies"),
			Namespace: "test",
			Labels:    map[string]string{model.GitDeployLabel: "true"},
		},
		Data: map[string]string{"variables": base64.StdEncoding.EncodeToString(stored)},
	}
	deployer := &fakeDeployer{}
	deployer.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(deployer, nil)
	deployer.On("Deploy", mock.Anything, mock.MatchedBy(func(opts *Options) bool {
		return opts.catalogSecretVariablesToForward == testCatalogSecretVariables && opts.Variables[0] == "OKTETO_CATALOG_SECRET_VARIABLES=[]"
	})).Return(nil)
	c := newCatalogRunCommand(t, catalogManifest(true), deployer, cfg)
	c.RunningInInstaller = true

	err = c.Run(context.Background(), &Options{Name: "movies", Namespace: "test", CatalogSecretVariables: testCatalogSecretVariables})

	require.NoError(t, err)
	deployer.AssertExpectations(t)
}
