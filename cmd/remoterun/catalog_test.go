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
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/catalog"
	"github.com/okteto/okteto/pkg/constants"
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

const (
	testInternalAPIIP                             = "10.20.30.40"
	testInternalAPIHostname                       = "okteto-internal-api.okteto.example"
	testCatalogSecretVariablesEndpoint            = "https://okteto-internal-api.okteto.example:8443/internal/catalog/secret-variables"
	testCatalogSecretVariablesIPEndpoint          = "https://10.20.30.40:8443/internal/catalog/secret-variables"
	testCatalogSecretVariablesToken               = "projected-token"
	internalAPIIPMissingCatalogSecretsError       = "the catalog secret variables can't be fetched: the remote build doesn't get the internal IP of the Okteto API to reach okteto-internal-api.okteto.example"
	internalAPIIPMissingHint                      = "Ask an Okteto admin to make sure the Okteto installation publishes the internal IP of the Okteto API, and that the Okteto CLI starting the remote build passes it on, "
	internalAPIHostnameMissingCatalogSecretsError = "the catalog secret variables can't be fetched: the remote build doesn't get the hostname of the internal API to reach 10.20.30.40"
	internalAPIHostnameMissingHint                = "Ask an Okteto admin to make sure the Okteto installation publishes the hostname of the internal API, and that the Okteto CLI starting the remote build passes it on, "
)

// testInternalAPI is what remote builds get from an installation that
// publishes both the internal IP of the Okteto API and the hostname of the
// internal API
var testInternalAPI = okteto.InternalAPI{Hostname: testInternalAPIHostname, IP: testInternalAPIIP}

// writeTestToken writes content to a token file in a temporary folder, and
// returns its path
func writeTestToken(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func newTestCatalogSecretVariables(t *testing.T, c catalogSecretVariablesGetter, phase catalog.Phase) catalogSecretVariables {
	t.Helper()
	return catalogSecretVariables{
		newClient:   func(string, okteto.InternalAPI, string) (catalogSecretVariablesGetter, error) { return c, nil },
		tokenFile:   writeTestToken(t, testCatalogSecretVariablesToken),
		action:      "action-name",
		endpoint:    testCatalogSecretVariablesEndpoint,
		internalAPI: testInternalAPI,
		phase:       phase,
	}
}

// recordingCatalogSecretVariablesClientFactory records the endpoints, the
// internal APIs and the tokens it creates clients with, and returns client
type recordingCatalogSecretVariablesClientFactory struct {
	client       catalogSecretVariablesGetter
	endpoints    []string
	internalAPIs []okteto.InternalAPI
	tokens       []string
}

func (r *recordingCatalogSecretVariablesClientFactory) newClient(endpoint string, api okteto.InternalAPI, token string) (catalogSecretVariablesGetter, error) {
	r.endpoints = append(r.endpoints, endpoint)
	r.internalAPIs = append(r.internalAPIs, api)
	r.tokens = append(r.tokens, token)
	return r.client, nil
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
	t.Setenv(constants.OktetoInternalAPIIPEnvVar, testInternalAPIIP)
	t.Setenv(constants.OktetoInternalAPIHostnameEnvVar, testInternalAPIHostname)

	got, err := newCatalogSecretVariables("", "", catalog.DeployPhase)

	require.NoError(t, err)
	require.Nil(t, got.newClient)
	require.Empty(t, got.tokenFile)
	require.Empty(t, got.internalAPI)
}

func TestNewCatalogSecretVariablesRequiresTheEndpoint(t *testing.T) {
	_, err := newCatalogSecretVariables("action-name", "", catalog.DeployPhase)

	require.EqualError(t, err, "--catalog-secret-variables-endpoint is required with --catalog-action")
}

func TestNewCatalogSecretVariablesForTheAction(t *testing.T) {
	t.Setenv(constants.OktetoInternalAPIIPEnvVar, testInternalAPIIP)
	t.Setenv(constants.OktetoInternalAPIHostnameEnvVar, testInternalAPIHostname)

	got, err := newCatalogSecretVariables("action-name", testCatalogSecretVariablesEndpoint, catalog.DestroyPhase)

	require.NoError(t, err)
	require.NotNil(t, got.newClient)
	require.Equal(t, "/run/secrets/catalog-secret-variables-token", got.tokenFile)
	require.Equal(t, "action-name", got.action)
	require.Equal(t, testCatalogSecretVariablesEndpoint, got.endpoint)
	require.Equal(t, testInternalAPI, got.internalAPI)
	require.Equal(t, catalog.DestroyPhase, got.phase)
}

// Neither the token is read nor the client built: a folder can't be read as a
// token file, and the factory records every client it creates
func TestCatalogSecretVariablesDoNotCallTheEndpointWithoutAValidInternalAPIIP(t *testing.T) {
	tests := []struct {
		name          string
		internalAPIIP string
	}{
		{name: "without internal API IP", internalAPIIP: ""},
		{name: "with an invalid internal API IP", internalAPIIP: "okteto-api.okteto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureJSONLogs(t)
			factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
			c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
			c.newClient = factory.newClient
			c.tokenFile = t.TempDir()
			c.internalAPI.IP = tt.internalAPIIP

			got, err := c.get(context.Background())

			require.ErrorAs(t, err, &internalAPIMissingError{})
			require.EqualError(t, err, internalAPIIPMissingCatalogSecretsError)
			require.Nil(t, got)
			require.Empty(t, factory.tokens)
			require.Equal(t, []oktetoLog.JSONLogFormat{
				catalogLog(oktetoLog.InfoLevel, internalAPIIPMissingHint+"then deploy again."),
				catalogLog(oktetoLog.ErrorLevel, internalAPIIPMissingCatalogSecretsError),
			}, jsonLogs(t, out))
		})
	}
}

// Neither the token is read nor the client built: a folder can't be read as a
// token file, and the factory records every client it creates
func TestCatalogSecretVariablesDoNotCallAnEndpointWithTheIPWithoutAValidHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
	}{
		{name: "without hostname", hostname: ""},
		{name: "with an invalid hostname", hostname: "okteto-internal-api.okteto.example:8443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureJSONLogs(t)
			factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
			c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
			c.newClient = factory.newClient
			c.tokenFile = t.TempDir()
			c.endpoint = testCatalogSecretVariablesIPEndpoint
			c.internalAPI.Hostname = tt.hostname

			got, err := c.get(context.Background())

			require.ErrorAs(t, err, &internalAPIMissingError{})
			require.EqualError(t, err, internalAPIHostnameMissingCatalogSecretsError)
			require.Nil(t, got)
			require.Empty(t, factory.tokens)
			require.Equal(t, []oktetoLog.JSONLogFormat{
				catalogLog(oktetoLog.InfoLevel, internalAPIHostnameMissingHint+"then deploy again."),
				catalogLog(oktetoLog.ErrorLevel, internalAPIHostnameMissingCatalogSecretsError),
			}, jsonLogs(t, out))
		})
	}
}

// Only the first installations with catalog secret variables give an http
// endpoint, which remote builds can't reach. Neither the token is read nor the
// client built
func TestCatalogSecretVariablesDoNotCallAnEndpointThatIsNotHTTPS(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "http", endpoint: "http://okteto-api.okteto:8080/internal/catalog/secret-variables"},
		{name: "without scheme", endpoint: "okteto-api.okteto/internal/catalog/secret-variables"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureJSONLogs(t)
			factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
			c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
			c.newClient = factory.newClient
			c.tokenFile = t.TempDir()
			c.endpoint = tt.endpoint

			got, err := c.get(context.Background())

			require.ErrorIs(t, err, errCatalogSecretVariablesEndpointNotHTTPS)
			require.Nil(t, got)
			require.Empty(t, factory.tokens)
			require.Equal(t, []oktetoLog.JSONLogFormat{
				catalogLog(oktetoLog.InfoLevel, "Ask an Okteto admin to upgrade the Okteto installation, then deploy again."),
				catalogLog(oktetoLog.ErrorLevel, "the catalog secret variables can't be fetched: the Okteto installation gives an endpoint that isn't https"),
			}, jsonLogs(t, out))
		})
	}
}

func TestCatalogSecretVariablesSkipsThemWithAnHTTPEndpointOnForceDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
	c := newTestCatalogSecretVariables(t, nil, catalog.DestroyPhase)
	c.newClient = factory.newClient
	c.endpoint = "http://okteto-api.okteto:8080/internal/catalog/secret-variables"
	c.forceDestroy = true

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Nil(t, got)
	require.Empty(t, factory.tokens)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogWarning("Running the destroy commands without the catalog secret variables: the catalog secret variables can't be fetched: the Okteto installation gives an endpoint that isn't https"),
	}, warnings(jsonLogs(t, out)))
}

func TestCatalogSecretVariablesCallsTheEndpointWithAnIPv6InternalAPIIP(t *testing.T) {
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(t, getter, catalog.DeployPhase)
	c.internalAPI.IP = "fd00::1"

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, &recordingCatalogSecretVariablesGetter{endpoint: testCatalogSecretVariablesEndpoint, action: "action-name"}, getter)
}

func TestCatalogSecretVariablesBuildsTheClientForTheEndpointAndTheInternalAPIIP(t *testing.T) {
	factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
	c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
	c.newClient = factory.newClient

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{testCatalogSecretVariablesEndpoint}, factory.endpoints)
	require.Equal(t, []okteto.InternalAPI{testInternalAPI}, factory.internalAPIs)
}

// Installations whose chart doesn't publish the hostname give an endpoint
// with it, which only needs the IP
func TestCatalogSecretVariablesGetsThemFromAnEndpointWithTheHostnameWithoutTheHostname(t *testing.T) {
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(t, getter, catalog.DeployPhase)
	c.internalAPI.Hostname = ""

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, &recordingCatalogSecretVariablesGetter{endpoint: testCatalogSecretVariablesEndpoint, action: "action-name"}, getter)
}

func TestCatalogSecretVariablesGetsThemFromAnEndpointWithTheIP(t *testing.T) {
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(t, getter, catalog.DeployPhase)
	c.endpoint = testCatalogSecretVariablesIPEndpoint
	c.internalAPI.IP = ""

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, &recordingCatalogSecretVariablesGetter{endpoint: testCatalogSecretVariablesIPEndpoint, action: "action-name"}, getter)
}

func TestCatalogSecretVariablesLogsTheClientError(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
	c.newClient = func(string, okteto.InternalAPI, string) (catalogSecretVariablesGetter, error) {
		return nil, errors.New("not logged in")
	}

	_, err := c.get(context.Background())

	require.EqualError(t, err, "not logged in")
	require.Equal(t, []oktetoLog.JSONLogFormat{catalogLog(oktetoLog.ErrorLevel, "not logged in")}, jsonLogs(t, out))
}

func TestCatalogSecretVariablesPointsToForceDestroyWithoutTheInternalAPIIPOnDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(t, &recordingCatalogSecretVariablesGetter{}, catalog.DestroyPhase)
	c.internalAPI.IP = ""

	_, err := c.get(context.Background())

	require.ErrorAs(t, err, &internalAPIMissingError{})
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.InfoLevel, internalAPIIPMissingHint+"then destroy again, or use force destroy to remove the environment even if the destroy commands can't run."))
}

func TestCatalogSecretVariablesSkipsThemWithoutTheInternalAPIIPOnForceDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(t, getter, catalog.DestroyPhase)
	c.internalAPI.IP = ""
	c.forceDestroy = true

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, &recordingCatalogSecretVariablesGetter{}, getter)
	require.Equal(t, []oktetoLog.JSONLogFormat{
		catalogWarning("Running the destroy commands without the catalog secret variables: " + internalAPIIPMissingCatalogSecretsError),
	}, warnings(jsonLogs(t, out)))
}

func TestCatalogSecretVariablesRejectsAnInvalidEndpoint(t *testing.T) {
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(t, getter, catalog.DeployPhase)
	c.endpoint = "https://okteto-internal-api.okteto.example:port/"

	_, err := c.get(context.Background())

	require.ErrorContains(t, err, "invalid catalog secret variables endpoint")
	require.Equal(t, &recordingCatalogSecretVariablesGetter{}, getter)
}

func TestCatalogSecretVariablesGetsThemForTheActionFromTheEndpoint(t *testing.T) {
	getter := &recordingCatalogSecretVariablesGetter{}
	c := newTestCatalogSecretVariables(t, getter, catalog.DeployPhase)

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, &recordingCatalogSecretVariablesGetter{endpoint: testCatalogSecretVariablesEndpoint, action: "action-name"}, getter)
}

// startTestInternalAPI starts the internal API with TLS, sets a current
// context with the user's token that trusts its certificate, and returns its
// endpoint, with a hostname the certificate is valid for, and its IP
func startTestInternalAPI(t *testing.T, handler http.HandlerFunc) (string, string) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	setTestOktetoContext(t)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	okteto.CurrentStore.Contexts["test"].Certificate = base64.StdEncoding.EncodeToString(certPEM)
	ip, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	return fmt.Sprintf("https://%s/internal/catalog/secret-variables", net.JoinHostPort("example.com", port)), ip
}

func TestNewCatalogSecretVariablesClientSendsTheTokenAsTheBearer(t *testing.T) {
	var authorization, action string
	endpoint, ip := startTestInternalAPI(t, func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		action = r.URL.Query().Get("action")
		_, _ = w.Write([]byte(`{"variables":[{"name":"PIN","value":"1"}]}`))
	})

	c, err := newCatalogSecretVariablesClient(endpoint, okteto.InternalAPI{IP: ip}, testCatalogSecretVariablesToken)
	require.NoError(t, err)
	got, err := c.GetCatalogSecretVariables(context.Background(), endpoint, "action-name")

	require.NoError(t, err)
	require.Equal(t, "Bearer projected-token", authorization)
	require.Equal(t, "action-name", action)
	require.Equal(t, []types.CatalogSecretVariable{{Name: "PIN", Value: "1"}}, got.Variables)
}

func TestNewCatalogSecretVariablesClientSendsTheUserTokenWithoutAToken(t *testing.T) {
	var authorization string
	endpoint, ip := startTestInternalAPI(t, func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	})

	c, err := newCatalogSecretVariablesClient(endpoint, okteto.InternalAPI{IP: ip}, "")
	require.NoError(t, err)
	_, err = c.GetCatalogSecretVariables(context.Background(), endpoint, "action-name")

	require.NoError(t, err)
	require.Equal(t, "Bearer token", authorization)
}

func TestCatalogSecretVariablesGetsThemWithTheTokenInTheFile(t *testing.T) {
	factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
	c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
	c.newClient = factory.newClient
	c.tokenFile = writeTestToken(t, "  projected-token\n")

	_, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{testCatalogSecretVariablesToken}, factory.tokens)
}

// Older installations don't provide the token, and their endpoint takes the user's
func TestCatalogSecretVariablesGetsThemWithTheUserTokenWithoutTheTokenFile(t *testing.T) {
	factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{
		CatalogSecretVariables: types.CatalogSecretVariables{Variables: []types.CatalogSecretVariable{{Name: "PIN", Value: "1"}}},
	}}
	c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
	c.newClient = factory.newClient
	c.tokenFile = filepath.Join(t.TempDir(), "missing")

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"PIN=1"}, got)
	require.Equal(t, []string{""}, factory.tokens)
}

func TestCatalogSecretVariablesFailsWhenTheTokenFileCannotBeRead(t *testing.T) {
	out := captureJSONLogs(t)
	factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
	c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
	c.newClient = factory.newClient
	// a folder can't be read as a file
	c.tokenFile = t.TempDir()

	got, err := c.get(context.Background())

	require.ErrorContains(t, err, "could not read the token to get the catalog secret variables: read "+c.tokenFile)
	require.Nil(t, got)
	require.Empty(t, factory.tokens)
	require.Equal(t, []oktetoLog.JSONLogFormat{catalogLog(oktetoLog.ErrorLevel, err.Error())}, jsonLogs(t, out))
}

func TestCatalogSecretVariablesFailsWithAnEmptyTokenFile(t *testing.T) {
	factory := &recordingCatalogSecretVariablesClientFactory{client: &client.FakeUserClient{}}
	c := newTestCatalogSecretVariables(t, nil, catalog.DeployPhase)
	c.newClient = factory.newClient
	c.tokenFile = writeTestToken(t, "\n")

	got, err := c.get(context.Background())

	require.EqualError(t, err, "could not read the token to get the catalog secret variables: "+c.tokenFile+" is empty")
	require.Nil(t, got)
	require.Empty(t, factory.tokens)
}

func TestCatalogSecretVariablesSkipsThemWithAnEmptyTokenFileOnForceDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{}, catalog.DestroyPhase)
	c.forceDestroy = true
	c.tokenFile = writeTestToken(t, "")

	got, err := c.get(context.Background())

	require.NoError(t, err)
	require.Nil(t, got)
	logged := warnings(jsonLogs(t, out))
	require.Len(t, logged, 1)
	require.Contains(t, logged[0].Message, "Running the destroy commands without the catalog secret variables: could not read the token to get the catalog secret variables")
}

func TestCatalogSecretVariablesNeverLogsTheToken(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
		ErrCatalogSecretVariables: errors.New("catalog secret variables request failed: 401 Unauthorized"),
	}, catalog.DeployPhase)

	_, err := c.get(context.Background())

	require.Error(t, err)
	require.NotContains(t, out.String(), testCatalogSecretVariablesToken)
}

// The commands run in the same RUN and can read the mounted token
func TestCatalogSecretVariablesMasksTheTokenInTheCommandsOutput(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{}, catalog.DeployPhase)

	_, err := c.get(context.Background())
	oktetoLog.EnableMasking()
	oktetoLog.SetStage("Deploy")
	oktetoLog.Println("the command printed " + testCatalogSecretVariablesToken)

	require.NoError(t, err)
	require.NotContains(t, out.String(), testCatalogSecretVariablesToken)
	require.Contains(t, out.String(), "the command printed ***")
}

// On an installation whose endpoint only takes the installer's token, the error says why it got the user's
func TestCatalogSecretVariablesSaysTheUserTokenWasUsedWhenTheRequestFails(t *testing.T) {
	out := captureJSONLogs(t)
	requestErr := errors.New("catalog secret variables request failed: 401 Unauthorized")
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{ErrCatalogSecretVariables: requestErr}, catalog.DeployPhase)
	c.tokenFile = filepath.Join(t.TempDir(), "missing")

	_, err := c.get(context.Background())

	require.ErrorIs(t, err, requestErr)
	require.EqualError(t, err, "catalog secret variables request failed: 401 Unauthorized ("+c.tokenFile+" isn't mounted, so the request used the user's token)")
	require.Contains(t, jsonLogs(t, out), catalogLog(oktetoLog.ErrorLevel, err.Error()))
}

func TestCatalogSecretVariablesReturnsTheVariables(t *testing.T) {
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
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
	c := newTestCatalogSecretVariables(t, withUnresolvedCatalogSecretVariables(), catalog.DeployPhase)

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
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
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
	c := newTestCatalogSecretVariables(t, withUnresolvedCatalogSecretVariables(), catalog.DestroyPhase)

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
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
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
			c := newTestCatalogSecretVariables(t, &client.FakeUserClient{ErrCatalogSecretVariables: tt.err}, catalog.DeployPhase)

			_, err := c.get(context.Background())

			require.ErrorIs(t, err, tt.err)
			require.Equal(t, []oktetoLog.JSONLogFormat{catalogLog(oktetoLog.ErrorLevel, tt.err.Error())}, jsonLogs(t, out))
		})
	}
}

func TestCatalogSecretVariablesSkipsThemAllWithInvalidNamesOnForceDestroy(t *testing.T) {
	out := captureJSONLogs(t)
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
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
			c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
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
	c := newTestCatalogSecretVariables(t, &client.FakeUserClient{
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
