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

package gitcredential

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

const (
	credentialsResponseBody = `{"username":"x-access-token","password":"token","expiresAt":"2099-01-01T00:00:00Z"}`

	internalAPIEndpoint   = "https://okteto-internal-api.example.com:8443/internal/git/credentials"
	internalAPIIP         = "10.20.30.40"
	internalAPIIPEndpoint = "https://10.20.30.40:8443/internal/git/credentials"
	// testContextName is the Okteto context in the env, whose hostname the helper checks the
	// certificate against for an endpoint with the IP
	testContextName = "https://okteto.example.com"

	// fakeEndpointIP is the IP httptest's TLS servers listen on
	fakeEndpointIP = "127.0.0.1"
)

// receivedRequest is what the fake endpoint got
type receivedRequest struct {
	method        string
	path          string
	host          string
	repository    string
	authorization string
}

// fakeEndpoint answers every request with status and body, and records the last request in received
func fakeEndpoint(t *testing.T, status int, body string, received *receivedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*received = receivedRequest{
			method:        r.Method,
			path:          r.URL.Path,
			host:          r.URL.Query().Get(hostQueryParam),
			repository:    r.URL.Query().Get(repositoryQueryParam),
			authorization: r.Header.Get("Authorization"),
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// fakeEndpointURL is the endpoint of the fake server on example.com, a hostname its certificate is
// valid for. The helper connects to fakeEndpointIP instead of resolving it
func fakeEndpointURL(t *testing.T, server *httptest.Server) string {
	t.Helper()
	return "https://example.com:" + mustParseURL(t, server.URL).Port() + "/internal/git/credentials"
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func setEndpointEnv(t *testing.T, endpoint, ip, token string) {
	t.Helper()
	t.Setenv(constants.OktetoGitCredentialsEndpointEnvVar, endpoint)
	t.Setenv(constants.OktetoInternalAPIIPEnvVar, ip)
	t.Setenv(model.OktetoTokenEnvVar, token)
	t.Setenv(model.OktetoURLEnvVar, "")
	t.Setenv(model.OktetoContextEnvVar, testContextName)
}

// unsetEnv unsets key during the test, so that a .env file could set it
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}

// fakeEndpointIPURL is the endpoint of the fake server with fakeEndpointIP as its host. The helper
// verifies the certificate against the hostname of the internal API it gets apart
func fakeEndpointIPURL(t *testing.T, server *httptest.Server) string {
	t.Helper()
	return "https://" + fakeEndpointIP + ":" + mustParseURL(t, server.URL).Port() + "/internal/git/credentials"
}

func TestNewGetterFromEnv(t *testing.T) {
	tests := []struct {
		expectedErr error
		name        string
		endpoint    string
		ip          string
		token       string
	}{
		{
			name:        "no endpoint",
			ip:          internalAPIIP,
			token:       "okteto-token",
			expectedErr: errEndpointNotSet,
		},
		{
			name:        "http endpoint",
			endpoint:    "http://okteto-internal-api.example.com:8080/internal/git/credentials",
			ip:          internalAPIIP,
			token:       "okteto-token",
			expectedErr: okteto.ErrInvalidInternalAPIEndpoint,
		},
		{
			name:        "endpoint without a scheme",
			endpoint:    "okteto-internal-api.example.com:8443/internal/git/credentials",
			ip:          internalAPIIP,
			token:       "okteto-token",
			expectedErr: okteto.ErrInvalidInternalAPIEndpoint,
		},
		{
			name:        "endpoint with another scheme",
			endpoint:    "ftp://okteto-internal-api.example.com/internal/git/credentials",
			ip:          internalAPIIP,
			token:       "okteto-token",
			expectedErr: okteto.ErrInvalidInternalAPIEndpoint,
		},
		{
			name:        "endpoint that isn't a URL",
			endpoint:    "https://okteto-internal-api.example.com:port/internal",
			ip:          internalAPIIP,
			token:       "okteto-token",
			expectedErr: okteto.ErrInvalidInternalAPIEndpoint,
		},
		{
			name:        "no internal IP of the API",
			endpoint:    internalAPIEndpoint,
			token:       "okteto-token",
			expectedErr: okteto.ErrInvalidInternalAPIIP,
		},
		{
			name:        "internal IP of the API that is a hostname",
			endpoint:    internalAPIEndpoint,
			ip:          "okteto-internal-api.example.com",
			token:       "okteto-token",
			expectedErr: okteto.ErrInvalidInternalAPIIP,
		},
		{
			name:        "no token",
			endpoint:    internalAPIEndpoint,
			ip:          internalAPIIP,
			expectedErr: errTokenNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEndpointEnv(t, tt.endpoint, tt.ip, tt.token)

			_, err := newGetterFromEnv()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestNewGetterFromEnv_HTTPSEndpointAndInternalIP(t *testing.T) {
	setEndpointEnv(t, internalAPIEndpoint, internalAPIIP, "okteto-token")

	_, err := newGetterFromEnv()

	require.NoError(t, err)
}

func TestNewGetterFromEnv_EndpointWithTheInternalIP(t *testing.T) {
	setEndpointEnv(t, internalAPIIPEndpoint, "", "okteto-token")

	_, err := newGetterFromEnv()

	require.NoError(t, err)
}

func TestNewGetterFromEnv_EndpointWithTheInternalIPWithoutAnOktetoContext(t *testing.T) {
	setEndpointEnv(t, internalAPIIPEndpoint, internalAPIIP, "okteto-token")
	t.Setenv(model.OktetoContextEnvVar, "")

	_, err := newGetterFromEnv()

	require.ErrorIs(t, err, okteto.ErrInvalidInternalAPIHostname)
	require.ErrorContains(t, err, "(OKTETO_URL or OKTETO_CONTEXT)")
}

// Loading the Okteto context would read the .env file of the repository Git runs the helper in,
// which could then pick where the token goes
func TestNewGetterFromEnv_DoesNotReadTheDotEnv(t *testing.T) {
	setEndpointEnv(t, internalAPIIPEndpoint, "", "okteto-token")
	unsetEnv(t, model.OktetoURLEnvVar)
	unsetEnv(t, model.OktetoContextEnvVar)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("OKTETO_URL=https://okteto.example.com\n"), 0600))
	t.Chdir(dir)

	_, err := newGetterFromEnv()

	require.ErrorIs(t, err, okteto.ErrInvalidInternalAPIHostname)
}

func TestContextNameFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		oktetoURL   string
		contextName string
		expected    string
	}{
		{name: "OKTETO_URL", oktetoURL: "https://okteto.example.com", expected: "https://okteto.example.com"},
		{name: "OKTETO_CONTEXT", contextName: "https://okteto.example.com", expected: "https://okteto.example.com"},
		{name: "OKTETO_URL first", oktetoURL: "https://okteto.example.com", contextName: "https://other.example.com", expected: "https://okteto.example.com"},
		{name: "neither"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(model.OktetoURLEnvVar, tt.oktetoURL)
			t.Setenv(model.OktetoContextEnvVar, tt.contextName)

			require.Equal(t, tt.expected, contextNameFromEnv())
		})
	}
}

func TestGetGitCredentials_SendsTheRepositoryAndTheToken(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, credentialsResponseBody, &received)
	getter, err := newGetter(fakeEndpointURL(t, server), okteto.InternalAPI{IP: fakeEndpointIP}, "okteto-token", server.Certificate())
	require.NoError(t, err)

	_, err = getter.GetGitCredentials(context.Background(), "github.com", "acme/infra")

	require.NoError(t, err)
	require.Equal(t, receivedRequest{
		method:        http.MethodGet,
		path:          "/internal/git/credentials",
		host:          "github.com",
		repository:    "acme/infra",
		authorization: "Bearer okteto-token",
	}, received)
}

func TestGetGitCredentials_ReadsTheCredentials(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected gitCredentials
	}{
		{
			name: "token with an expiry",
			body: credentialsResponseBody,
			expected: gitCredentials{
				Username:  "x-access-token",
				Password:  "token",
				ExpiresAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:     "token without an expiry",
			body:     `{"username":"x-access-token","password":"token"}`,
			expected: gitCredentials{Username: "x-access-token", Password: "token"},
		},
		{
			name:     "expiry that can't be read keeps the token",
			body:     `{"username":"x-access-token","password":"token","expiresAt":"in an hour"}`,
			expected: gitCredentials{Username: "x-access-token", Password: "token"},
		},
		{
			name:     "only a reason",
			body:     `{"reason":"link your GitHub account in Okteto, in Settings → Integrations"}`,
			expected: gitCredentials{Reason: "link your GitHub account in Okteto, in Settings → Integrations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received receivedRequest
			server := fakeEndpoint(t, http.StatusOK, tt.body, &received)
			getter := &endpointClient{httpClient: server.Client(), endpoint: mustParseURL(t, server.URL), token: "okteto-token"}

			creds, err := getter.GetGitCredentials(context.Background(), "github.com", "acme/infra")

			require.NoError(t, err)
			require.Equal(t, tt.expected, creds)
		})
	}
}

func TestGetGitCredentials_ErrorStatusIsNoCredentials(t *testing.T) {
	tests := []struct {
		name        string
		expectedErr string
		status      int
	}{
		{name: "bad repository", status: http.StatusBadRequest, expectedErr: "the Okteto API answered 400 Bad Request"},
		{name: "caller not accepted", status: http.StatusUnauthorized, expectedErr: "the Okteto API answered 401 Unauthorized"},
		{name: "older Okteto without the endpoint", status: http.StatusNotFound, expectedErr: "the Okteto API answered 404 Not Found"},
		{name: "method not allowed", status: http.StatusMethodNotAllowed, expectedErr: "the Okteto API answered 405 Method Not Allowed"},
		{name: "server error", status: http.StatusInternalServerError, expectedErr: "the Okteto API answered 500 Internal Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received receivedRequest
			server := fakeEndpoint(t, tt.status, "", &received)
			getter := &endpointClient{httpClient: server.Client(), endpoint: mustParseURL(t, server.URL), token: "okteto-token"}

			_, err := getter.GetGitCredentials(context.Background(), "github.com", "acme/infra")

			require.EqualError(t, err, tt.expectedErr)
		})
	}
}

func TestGetGitCredentials_AnswerThatIsNotJSON(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, "<html>token</html>", &received)
	getter := &endpointClient{httpClient: server.Client(), endpoint: mustParseURL(t, server.URL), token: "okteto-token"}

	_, err := getter.GetGitCredentials(context.Background(), "github.com", "acme/infra")

	require.EqualError(t, err, "could not read the answer of the Okteto API")
}

func TestGetGitCredentials_AnswerOverTheSizeLimit(t *testing.T) {
	var received receivedRequest
	padding := strings.Repeat(" ", maxResponseSize)
	server := fakeEndpoint(t, http.StatusOK, padding+credentialsResponseBody, &received)
	getter := &endpointClient{httpClient: server.Client(), endpoint: mustParseURL(t, server.URL), token: "okteto-token"}

	_, err := getter.GetGitCredentials(context.Background(), "github.com", "acme/infra")

	require.EqualError(t, err, "could not read the answer of the Okteto API")
}

func TestGetGitCredentials_UnreachableEndpoint(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, credentialsResponseBody, &received)
	getter := &endpointClient{httpClient: server.Client(), endpoint: mustParseURL(t, server.URL), token: "okteto-token"}
	server.Close()

	_, err := getter.GetGitCredentials(context.Background(), "github.com", "acme/infra")

	require.ErrorContains(t, err, "could not reach the Okteto API")
}

func TestGet_FromTheEndpoint(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, credentialsResponseBody, &received)
	provider := func() (credentialsGetter, error) {
		return newGetter(fakeEndpointURL(t, server), okteto.InternalAPI{IP: fakeEndpointIP}, "okteto-token", server.Certificate())
	}

	stdout, stderr := runGet(githubRequest, provider)

	require.Equal(t, "username=x-access-token\npassword=token\npassword_expiry_utc=4070908800\n", stdout)
	require.Empty(t, stderr)
}

func TestGet_FromAnEndpointWithTheInternalIP(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, credentialsResponseBody, &received)
	provider := func() (credentialsGetter, error) {
		return newGetter(fakeEndpointIPURL(t, server), okteto.InternalAPI{Hostname: "example.com"}, "okteto-token", server.Certificate())
	}

	stdout, stderr := runGet(githubRequest, provider)

	require.Equal(t, "username=x-access-token\npassword=token\npassword_expiry_utc=4070908800\n", stdout)
	require.Empty(t, stderr)
	require.Equal(t, "Bearer okteto-token", received.authorization)
}

func TestGet_WithoutTheEndpointEnvPrintsNothing(t *testing.T) {
	setEndpointEnv(t, "", internalAPIIP, "okteto-token")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stdout)
	require.Empty(t, stderr)
}

func TestGet_WithoutTheInternalIPSendsNoToken(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, credentialsResponseBody, &received)
	setEndpointEnv(t, fakeEndpointURL(t, server), "", "okteto-token")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stdout)
	require.Equal(t, "okteto: no GitHub credentials for acme/infra: the internal IP of the Okteto API is missing or invalid (OKTETO_INTERNAL_API_IP)\n", stderr)
	require.Equal(t, receivedRequest{}, received)
}

func TestGet_WithoutTheOktetoContextSendsNoToken(t *testing.T) {
	var received receivedRequest
	server := fakeEndpoint(t, http.StatusOK, credentialsResponseBody, &received)
	setEndpointEnv(t, fakeEndpointIPURL(t, server), "", "okteto-token")
	t.Setenv(model.OktetoContextEnvVar, "")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stdout)
	require.Equal(t, "okteto: no GitHub credentials for acme/infra: the hostname of the internal API is missing or invalid (OKTETO_URL or OKTETO_CONTEXT)\n", stderr)
	require.Equal(t, receivedRequest{}, received)
}
