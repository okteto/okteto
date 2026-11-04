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

func newTestCatalogSecretVariables(c *client.FakeUserClient, retryHint string) catalogSecretVariables {
	return catalogSecretVariables{
		client:    c,
		itemID:    "item",
		endpoint:  "https://okteto.example/internal/catalog/secret-variables",
		retryHint: retryHint,
	}
}

var unresolvedCatalogSecretVariables = okteto.UnresolvedCatalogSecretVariablesError{
	Variables: []okteto.UnresolvedCatalogSecretVariable{
		{Variable: "API_TOKEN", SecretKeyRef: okteto.CatalogSecretKeyRef{Name: "creds", Key: "token"}, Reason: okteto.CatalogSecretNotFoundReason},
		{Variable: "DB_PASSWORD", SecretKeyRef: okteto.CatalogSecretKeyRef{Name: "db", Key: "password"}, Reason: okteto.CatalogSecretKeyNotFoundReason},
		{Variable: "NPM_TOKEN", SecretKeyRef: okteto.CatalogSecretKeyRef{Name: "npm", Key: "token"}, Reason: okteto.CatalogSecretNotLabeledReason},
	},
}

func TestCatalogSecretVariablesWithoutCatalogItem(t *testing.T) {
	got, err := catalogSecretVariables{}.get(context.Background())

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestNewCatalogSecretVariablesWithoutCatalogItem(t *testing.T) {
	got, err := newCatalogSecretVariables("", "http://endpoint", deployRetryHint)

	require.NoError(t, err)
	require.Nil(t, got.client)
}

func TestNewCatalogSecretVariablesRequiresTheEndpoint(t *testing.T) {
	_, err := newCatalogSecretVariables("item", "", deployRetryHint)

	require.EqualError(t, err, "--catalog-secret-variables-endpoint is required with --catalog-item-id")
}

func TestCatalogSecretVariablesReturnsTheVariables(t *testing.T) {
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: []types.CatalogSecretVariable{
			{Name: "API_TOKEN", Value: "abc=def"},
			{Name: "PIN", Value: "1"},
		},
	}, deployRetryHint)

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"API_TOKEN=abc=def", "PIN=1"}, got)
}

func TestCatalogSecretVariablesLogsUnresolvedVariablesOnDeploy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: unresolvedCatalogSecretVariables}, deployRetryHint)

	got, err := c.get(context.Background())

	require.ErrorAs(t, err, &okteto.UnresolvedCatalogSecretVariablesError{})
	require.Nil(t, got)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogLog(oktetoLog.InfoLevel, `Catalog variable "API_TOKEN" could not be resolved: Secret "creds" not found`),
		catalogLog(oktetoLog.InfoLevel, `Catalog variable "DB_PASSWORD" could not be resolved: key "password" not found in Secret "db"`),
		catalogLog(oktetoLog.InfoLevel, `Catalog variable "NPM_TOKEN" could not be resolved: Secret "npm" isn't labelled dev.okteto.com/catalog-secret=true`),
		catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to make sure the Secret exists in the Okteto namespace with the label dev.okteto.com/catalog-secret=true, or to fix the reference in the catalog item, then deploy again."),
		catalogLog(oktetoLog.ErrorLevel, "the secret variables of the catalog item could not be resolved"),
	}, jsonLogs(t, out))
}

func TestCatalogSecretVariablesLogsUnresolvedVariablesOnDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: unresolvedCatalogSecretVariables}, destroyRetryHint)

	_, err := c.get(context.Background())

	require.Error(t, err)
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to make sure the Secret exists in the Okteto namespace with the label dev.okteto.com/catalog-secret=true, or to fix the reference in the catalog item, then destroy again, or use force destroy to remove the environment even if the destroy commands can't run."))
}

func TestCatalogSecretVariablesLogsUnknownReasons(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		ErrCatalogSecretVariables: okteto.UnresolvedCatalogSecretVariablesError{
			Variables: []okteto.UnresolvedCatalogSecretVariable{{Variable: "API_TOKEN", Reason: "other_reason"}},
		},
	}, deployRetryHint)

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
			c := newTestCatalogSecretVariables(&client.FakeUserClient{ErrCatalogSecretVariables: tt.err}, deployRetryHint)

			_, err := c.get(context.Background())

			require.ErrorIs(t, err, tt.err)
			require.Equal(t, []oktetoLog.JSONLogFormat{catalogLog(oktetoLog.ErrorLevel, tt.err.Error())}, jsonLogs(t, out))
		})
	}
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
				CatalogSecretVariables: []types.CatalogSecretVariable{
					{Name: "VALID", Value: "value"},
					{Name: tt.variable, Value: "value"},
				},
			}, deployRetryHint)

			got, err := c.get(context.Background())

			require.ErrorAs(t, err, &invalidCatalogSecretVariableNamesError{})
			require.Nil(t, got)
			require.Equal(t, []oktetoLog.JSONLogFormat{
				catalogLog(oktetoLog.InfoLevel, tt.expected),
				catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to rename the variable in the catalog item, then deploy again."),
				catalogLog(oktetoLog.ErrorLevel, "the secret variables of the catalog item have invalid names"),
			}, jsonLogs(t, out))
		})
	}
}

func TestCatalogSecretVariablesNeverLogsTheValues(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(&client.FakeUserClient{
		CatalogSecretVariables: []types.CatalogSecretVariable{
			{Name: "API_TOKEN", Value: "secret-value"},
			{Name: "OKTETO_TOKEN", Value: "other-secret-value"},
		},
	}, deployRetryHint)

	_, err := c.get(context.Background())

	require.Error(t, err)
	require.NotContains(t, out.String(), "secret-value")
}
