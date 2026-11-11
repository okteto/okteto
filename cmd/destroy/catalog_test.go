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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/catalog"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
)

var testCatalogSecretVariables = catalog.SecretVariables{
	References: `[{"name":"API_TOKEN","secretKeyRef":{"name":"creds","key":"token"}}]`,
	Action:     "action-name",
	Endpoint:   "https://okteto.example/internal/catalog/secret-variables",
}

// catalogSecretVariablesWithReferences returns the test catalog secret variables with the references provided
func catalogSecretVariablesWithReferences(references string) catalog.SecretVariables {
	s := testCatalogSecretVariables
	s.References = references
	return s
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

// catalogDestroy is a destroy command that runs its commands locally with executor
type catalogDestroy struct {
	cmd       *destroyCommand
	client    kubernetes.Interface
	destroyer *fakeDestroyer
	executor  *fakeExecutor
}

func newCatalogDestroy(t *testing.T) *catalogDestroy {
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
	cd := newCatalogDestroy(t)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(false),
		CatalogSecretVariables: testCatalogSecretVariables,
	})

	require.ErrorIs(t, err, catalog.ErrSecretVariablesNeedRemote)
	require.Empty(t, cd.executor.executed)
	require.False(t, cd.destroyer.destroyed)
	cd.requireStatus(t, pipeline.ErrorStatus)
}

func TestDestroyFailsBeforeAnyCommandWhenCatalogSecretVariablesCannotBeRead(t *testing.T) {
	cd := newCatalogDestroy(t)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(true),
		CatalogSecretVariables: catalogSecretVariablesWithReferences(`{"name":`),
	})

	require.ErrorContains(t, err, "could not read the catalog secret variables")
	require.Empty(t, cd.executor.executed)
	require.False(t, cd.destroyer.destroyed)
	cd.requireStatus(t, pipeline.ErrorStatus)
}

// captureJSONLogs sends the okteto logger output to the returned buffer in json, which drops the lines logged without
// a stage, until the test finishes. The stage starts empty, as other tests leave theirs set
func captureJSONLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	oktetoLog.SetStage("")
	oktetoLog.Init(logrus.InfoLevel)
	oktetoLog.SetOutputFormat(oktetoLog.JSONFormat)
	oktetoLog.SetOutput(out)
	t.Cleanup(func() {
		oktetoLog.SetStage("")
		oktetoLog.DisableMasking()
		oktetoLog.Init(logrus.InfoLevel)
	})
	return out
}

// catalogStageLogs returns the json lines logged to out under the stage of the catalog secret variables, without
// their timestamp
func catalogStageLogs(out *bytes.Buffer) []oktetoLog.JSONLogFormat {
	var lines []oktetoLog.JSONLogFormat
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		var line oktetoLog.JSONLogFormat
		if json.Unmarshal(scanner.Bytes(), &line) != nil || line.Stage != catalog.SecretVariablesStage {
			continue
		}
		line.Timestamp = 0
		lines = append(lines, line)
	}
	return lines
}

func TestDestroyLogsTheFailedCatalogSecretVariablesCheckUnderItsStage(t *testing.T) {
	cd := newCatalogDestroy(t)
	out := captureJSONLogs(t)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(false),
		CatalogSecretVariables: testCatalogSecretVariables,
	})

	require.ErrorIs(t, err, catalog.ErrSecretVariablesNeedRemote)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		{Level: oktetoLog.InfoLevel, Stage: catalog.SecretVariablesStage, Message: "the development environment has catalog secret variables: API_TOKEN"},
		{Level: oktetoLog.InfoLevel, Stage: catalog.SecretVariablesStage, Message: "Destruction failed: " + catalog.ErrSecretVariablesNeedRemote.Error()},
	}, catalogStageLogs(out))
	require.Equal(t, catalog.SecretVariablesStage, oktetoLog.GetStage())
}

func TestForceDestroyWarnsUnderTheCatalogSecretVariablesStageWhenTheCheckFails(t *testing.T) {
	cd := newCatalogDestroy(t)
	out := captureJSONLogs(t)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(false),
		CatalogSecretVariables: testCatalogSecretVariables,
		ForceDestroy:           true,
	})

	require.NoError(t, err)
	require.Contains(t, catalogStageLogs(out), oktetoLog.JSONLogFormat{
		Level:   oktetoLog.WarningLevel,
		Stage:   catalog.SecretVariablesStage,
		Message: " !  Running the destroy commands without the catalog secret variables: " + catalog.ErrSecretVariablesNeedRemote.Error(),
	})
}

func TestForceDestroyForwardsTheCatalogSecretVariablesWhenTheCheckPasses(t *testing.T) {
	opts := &Options{
		Name:                   "test-app",
		Manifest:               catalogManifest(true),
		CatalogSecretVariables: testCatalogSecretVariables,
		ForceDestroy:           true,
	}

	err := checkCatalogSecretVariables(opts)

	require.NoError(t, err)
	require.Equal(t, testCatalogSecretVariables, opts.catalogSecretVariablesToForward)
}

func TestForceDestroyRunsTheCommandsWithoutTheCatalogSecretVariablesWhenTheCheckFails(t *testing.T) {
	cd := newCatalogDestroy(t)
	opts := &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(false),
		CatalogSecretVariables: testCatalogSecretVariables,
		ForceDestroy:           true,
	}

	err := cd.cmd.destroy(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, catalog.SecretVariables{}, opts.catalogSecretVariablesToForward)
	require.Len(t, cd.executor.executed, 1)
	require.True(t, cd.destroyer.destroyed)
}

func TestDestroyLocallyWithoutCatalogSecretVariables(t *testing.T) {
	cd := newCatalogDestroy(t)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(false),
		CatalogSecretVariables: catalogSecretVariablesWithReferences("[]"),
	})

	require.NoError(t, err)
	require.Len(t, cd.executor.executed, 1)
	require.True(t, cd.destroyer.destroyed)
}

func TestDestroyWithoutCatalogSecretVariablesDoesNotForwardThem(t *testing.T) {
	cd := newCatalogDestroy(t)
	opts := &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               catalogManifest(false),
		CatalogSecretVariables: catalog.SecretVariables{Action: "action-name"},
	}

	err := cd.cmd.destroy(context.Background(), opts)

	require.NoError(t, err)
	require.Equal(t, catalog.SecretVariables{}, opts.catalogSecretVariablesToForward)
	require.Len(t, cd.executor.executed, 1)
}

func TestDestroyWithoutDestroyCommandsDoesNotCheckCatalogSecretVariables(t *testing.T) {
	cd := newCatalogDestroy(t)

	err := cd.cmd.destroy(context.Background(), &Options{
		Name:                   "test-app",
		Namespace:              "namespace",
		Manifest:               &model.Manifest{Name: "test-app", Destroy: &model.DestroyInfo{}},
		CatalogSecretVariables: catalogSecretVariablesWithReferences(`{"name":`),
	})

	require.NoError(t, err)
	require.True(t, cd.destroyer.destroyed)
}

func TestCheckCatalogSecretVariablesForwardsThemWhenDestroyRunsRemotely(t *testing.T) {
	opts := &Options{
		Name:                   "test-app",
		Manifest:               catalogManifest(true),
		CatalogSecretVariables: testCatalogSecretVariables,
	}

	err := checkCatalogSecretVariables(opts)

	require.NoError(t, err)
	require.Equal(t, testCatalogSecretVariables, opts.catalogSecretVariablesToForward)
}

func TestReadCatalogSecretVariablesReadsThemFromTheEnv(t *testing.T) {
	t.Setenv(catalog.SecretVariablesEnvVar, testCatalogSecretVariables.References)
	t.Setenv(model.OktetoActionNameEnvVar, testCatalogSecretVariables.Action)
	t.Setenv(catalog.SecretVariablesEndpointEnvVar, testCatalogSecretVariables.Endpoint)
	options := &Options{}

	readCatalogSecretVariables(options)

	require.Equal(t, testCatalogSecretVariables, options.CatalogSecretVariables)
}
