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

package remoterun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/catalog"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// captureJSONLogs sends the okteto logger output to the returned buffer in
// json, the format remote-run uses, until the test finishes
func captureJSONLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
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

// jsonLogs returns the json lines logged to out, without the timestamp
func jsonLogs(t *testing.T, out *bytes.Buffer) []oktetoLog.JSONLogFormat {
	t.Helper()
	var lines []oktetoLog.JSONLogFormat
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		var line oktetoLog.JSONLogFormat
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &line))
		line.Timestamp = 0
		lines = append(lines, line)
	}
	return lines
}

func catalogLog(level, message string) oktetoLog.JSONLogFormat {
	return oktetoLog.JSONLogFormat{Level: level, Stage: catalogSecretVariablesStage, Message: message}
}

// catalogWarning is a warning logged under the stage of the catalog secret
// variables
func catalogWarning(message string) oktetoLog.JSONLogFormat {
	return catalogLog("warn", " !  "+message)
}

// warnings returns the warnings in lines. Warnings are also logged without a
// stage at info level, and the deploy or destroy running remote-run drops
// those lines
func warnings(lines []oktetoLog.JSONLogFormat) []oktetoLog.JSONLogFormat {
	var result []oktetoLog.JSONLogFormat
	for _, line := range lines {
		if line.Level == "warn" {
			result = append(result, line)
		}
	}
	return result
}

const testCatalogSecretVariablesEndpoint = "https://okteto.example/internal/catalog/secret-variables"

func newTestCatalogSecretVariables(c catalogSecretVariablesGetter, phase catalog.Phase) catalogSecretVariables {
	return catalogSecretVariables{
		client:   c,
		action:   "action-name",
		endpoint: testCatalogSecretVariablesEndpoint,
		phase:    phase,
	}
}

// recordingCatalogSecretVariablesGetter records the endpoint and the action it
// gets the catalog secret variables for
type recordingCatalogSecretVariablesGetter struct {
	endpoint string
	action   string
}

func (r *recordingCatalogSecretVariablesGetter) GetCatalogSecretVariables(_ context.Context, endpoint, action string) (types.CatalogSecretVariables, error) {
	r.endpoint = endpoint
	r.action = action
	return types.CatalogSecretVariables{}, nil
}

// withUnresolvedCatalogSecretVariables returns a client whose response has
// PIN resolved and three unresolved variables, one per reason
func withUnresolvedCatalogSecretVariables() *client.FakeUserClient {
	return &client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{
			Variables: []types.CatalogSecretVariable{{Name: "PIN", Value: "1"}},
			Unresolved: []types.UnresolvedCatalogSecretVariable{
				{Variable: "API_TOKEN", SecretKeyRef: types.CatalogSecretKeyRef{Name: "creds", Key: "token"}, Reason: types.CatalogSecretNotFoundReason},
				{Variable: "DB_PASSWORD", SecretKeyRef: types.CatalogSecretKeyRef{Name: "db", Key: "password"}, Reason: types.CatalogSecretKeyNotFoundReason},
				{Variable: "NPM_TOKEN", SecretKeyRef: types.CatalogSecretKeyRef{Name: "npm", Key: "token"}, Reason: types.CatalogSecretNotLabeledReason},
			},
		},
	}
}

func TestCatalogSecretVariablesWithoutAction(t *testing.T) {
	got, err := catalogSecretVariables{}.get(context.Background())

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestNewCatalogSecretVariablesWithoutAction(t *testing.T) {
	got, err := newCatalogSecretVariables("", "http://endpoint", catalog.DeployPhase)

	require.NoError(t, err)
	require.Nil(t, got.client)
}

func TestNewCatalogSecretVariablesRequiresTheEndpoint(t *testing.T) {
	_, err := newCatalogSecretVariables("action-name", "", catalog.DeployPhase)

	require.EqualError(t, err, "--catalog-secret-variables-endpoint is required with --catalog-action")
}

func TestNewCatalogSecretVariablesForTheAction(t *testing.T) {
	setTestOktetoContext(t)

	got, err := newCatalogSecretVariables("action-name", testCatalogSecretVariablesEndpoint, catalog.DestroyPhase)

	require.NoError(t, err)
	require.NotNil(t, got.client)
	require.Equal(t, "action-name", got.action)
	require.Equal(t, testCatalogSecretVariablesEndpoint, got.endpoint)
	require.Equal(t, catalog.DestroyPhase, got.phase)
}

func TestCatalogSecretVariablesGetsThemForTheActionFromTheEndpoint(t *testing.T) {
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(getter, catalog.DeployPhase)

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, &recordingCatalogSecretVariablesGetter{endpoint: testCatalogSecretVariablesEndpoint, action: "action-name"}, getter)
}

func TestCatalogSecretVariablesReturnsTheVariables(t *testing.T) {
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{
			Variables: []types.CatalogSecretVariable{
				{Name: "API_TOKEN", Value: "abc=def"},
				{Name: "PIN", Value: "1"},
			},
		},
	}, catalog.DeployPhase)

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"API_TOKEN=abc=def", "PIN=1"}, got)
}

func TestCatalogSecretVariablesFailsWithUnresolvedVariablesOnDeploy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(withUnresolvedCatalogSecretVariables(), catalog.DeployPhase)

	got, err := c.get(context.Background())

	require.ErrorAs(t, err, &unresolvedCatalogSecretVariablesError{})
	require.Nil(t, got)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogLog(oktetoLog.InfoLevel, `Catalog variable "API_TOKEN" could not be resolved: Secret "creds" not found`),
		catalogLog(oktetoLog.InfoLevel, `Catalog variable "DB_PASSWORD" could not be resolved: key "password" not found in Secret "db"`),
		catalogLog(oktetoLog.InfoLevel, `Catalog variable "NPM_TOKEN" could not be resolved: Secret "npm" isn't labelled dev.okteto.com/catalog-secret=true`),
		catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to make sure the Secret exists in the Okteto namespace with the label dev.okteto.com/catalog-secret=true, or to fix the reference in the catalog item, then deploy again."),
		catalogLog(oktetoLog.ErrorLevel, "the catalog secret variables could not be resolved"),
	}, jsonLogs(t, out))
}

func TestCatalogSecretVariablesEndsTheHintWithTheDestroyRetry(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{
			Variables: []types.CatalogSecretVariable{{Name: "OKTETO_TOKEN", Value: "value"}},
		},
	}, catalog.DestroyPhase)

	_, err := c.get(context.Background())

	require.Error(t, err)
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to rename the variable in the catalog item, then destroy again, or use force destroy to remove the environment even if the destroy commands can't run."))
}

func TestCatalogSecretVariablesSkipsUnresolvedVariablesOnDestroyWithAWarningEach(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(withUnresolvedCatalogSecretVariables(), catalog.DestroyPhase)

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"PIN=1"}, got)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogWarning(`Catalog variable "API_TOKEN" could not be resolved: Secret "creds" not found. The commands run without it`),
		catalogWarning(`Catalog variable "DB_PASSWORD" could not be resolved: key "password" not found in Secret "db". The commands run without it`),
		catalogWarning(`Catalog variable "NPM_TOKEN" could not be resolved: Secret "npm" isn't labelled dev.okteto.com/catalog-secret=true. The commands run without it`),
	}, warnings(jsonLogs(t, out)))
}

func TestCatalogSecretVariablesLogsUnknownReasons(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{
			Unresolved: []types.UnresolvedCatalogSecretVariable{{Variable: "API_TOKEN", Reason: "other_reason"}},
		},
	}, catalog.DeployPhase)

	_, err := c.get(context.Background())

	require.Error(t, err)
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.InfoLevel, `Catalog variable "API_TOKEN" could not be resolved: other_reason`))
}

func TestCatalogSecretVariablesLogsOtherErrors(t *testing.T) {
	tests := []struct {
		err  error
		name string
	}{
		{name: "feature disabled", err: okteto.ErrCatalogSecretVariablesDisabled},
		{name: "request failed", err: errors.New("catalog secret variables request failed: 401 Unauthorized")},
		{name: "unreachable", err: errors.New("could not reach the catalog secret variables endpoint: connection refused")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureJSONLogs(t)
			c := newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: tt.err}, catalog.DeployPhase)

			_, err := c.get(context.Background())

			require.ErrorIs(t, err, tt.err)
			require.Equal(t, []oktetoLog.JSONLogFormat{catalogLog(oktetoLog.ErrorLevel, tt.err.Error())}, jsonLogs(t, out))
		})
	}
}

func TestCatalogSecretVariablesSkipsThemAllWithInvalidNamesOnForceDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{
			Variables: []types.CatalogSecretVariable{
				{Name: "VALID", Value: "value"},
				{Name: "OKTETO_TOKEN", Value: "value"},
			},
		},
	}, catalog.DestroyPhase)
	c.forceDestroy = true

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogWarning(`Catalog variable "OKTETO_TOKEN" can't be injected: names starting with OKTETO_ are reserved`),
		catalogWarning("Running the destroy commands without the catalog secret variables: the catalog secret variables have invalid names"),
	}, warnings(jsonLogs(t, out)))
}

func TestCatalogSecretVariablesRejectsInvalidNames(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		expected string
	}{
		{name: "okteto variable", variable: "OKTETO_TOKEN", expected: `Catalog variable "OKTETO_TOKEN" can't be injected: names starting with OKTETO_ are reserved`},
		{name: "equal sign", variable: "A=B", expected: `Catalog variable "A=B" can't be injected: it isn't a valid environment variable name`},
		{name: "leading digit", variable: "1A", expected: `Catalog variable "1A" can't be injected: it isn't a valid environment variable name`},
		{name: "empty", variable: "", expected: `Catalog variable "" can't be injected: it isn't a valid environment variable name`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureJSONLogs(t)
			c := newTestCatalogSecretVariables(&client.FakeUserClient{
				CatalogSecretVariables: types.CatalogSecretVariables{
					Variables: []types.CatalogSecretVariable{
						{Name: "VALID", Value: "value"},
						{Name: tt.variable, Value: "value"},
					},
				},
			}, catalog.DeployPhase)

			got, err := c.get(context.Background())

			require.ErrorAs(t, err, &invalidCatalogSecretVariableNamesError{})
			require.Nil(t, got)
			require.Equal(t, []oktetoLog.JSONLogFormat{
				catalogLog(oktetoLog.InfoLevel, tt.expected),
				catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to rename the variable in the catalog item, then deploy again."),
				catalogLog(oktetoLog.ErrorLevel, "the catalog secret variables have invalid names"),
			}, jsonLogs(t, out))
		})
	}
}

func TestCatalogSecretVariablesNeverLogsTheValues(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{
			Variables: []types.CatalogSecretVariable{
				{Name: "API_TOKEN", Value: "secret-value"},
				{Name: "OKTETO_TOKEN", Value: "other-secret-value"},
			},
		},
	}, catalog.DeployPhase)

	_, err := c.get(context.Background())

	require.Error(t, err)
	require.NotContains(t, out.String(), "secret-value")
}
