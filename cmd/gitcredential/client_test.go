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
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
)

const gitCredentialsResponse = `{"data":{"gitCredentials":{"username":"x-access-token","password":"token","expiresAt":"2026-11-13T10:00:00Z","reason":null}}}`

func TestAPIConfigFromEnv(t *testing.T) {
	tests := []struct {
		expectedErr error
		name        string
		url         string
		context     string
		token       string
		serverName  string
		expected    apiConfig
	}{
		{
			name:     "OKTETO_URL",
			url:      "https://okteto.example.com",
			token:    "okteto-token",
			expected: apiConfig{url: "https://okteto.example.com", token: "okteto-token"},
		},
		{
			name:       "internal server name in remote runs",
			url:        "https://okteto.example.com",
			token:      "okteto-token",
			serverName: "10.0.0.1:443",
			expected:   apiConfig{url: "https://okteto.example.com", token: "okteto-token", serverName: "10.0.0.1:443"},
		},
		{
			name:     "OKTETO_CONTEXT when OKTETO_URL is not set",
			context:  "https://okteto.example.com",
			token:    "okteto-token",
			expected: apiConfig{url: "https://okteto.example.com", token: "okteto-token"},
		},
		{
			name:     "OKTETO_URL first, as in the rest of the CLI",
			url:      "https://okteto.example.com",
			context:  "https://other.example.com",
			token:    "okteto-token",
			expected: apiConfig{url: "https://okteto.example.com", token: "okteto-token"},
		},
		{
			name:        "no Okteto URL",
			token:       "okteto-token",
			expectedErr: errURLNotSet,
		},
		{
			name:        "no token",
			url:         "https://okteto.example.com",
			expectedErr: errTokenNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(model.OktetoURLEnvVar, tt.url)
			t.Setenv(model.OktetoContextEnvVar, tt.context)
			t.Setenv(model.OktetoTokenEnvVar, tt.token)
			t.Setenv(constants.OktetoInternalServerNameEnvVar, tt.serverName)

			cfg, err := apiConfigFromEnv()

			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expected, cfg)
		})
	}
}

func TestCertificateFor(t *testing.T) {
	store := &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"https://okteto.example.com": {Name: "https://okteto.example.com", Certificate: "cert"},
		},
	}

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with a scheme",
			url:      "https://okteto.example.com",
			expected: "cert",
		},
		{
			name:     "URL with a trailing slash",
			url:      "https://okteto.example.com/",
			expected: "cert",
		},
		{
			name:     "URL without a scheme",
			url:      "okteto.example.com",
			expected: "cert",
		},
		{
			name:     "instance without a context",
			url:      "https://other.example.com",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, certificateFor(store, tt.url))
		})
	}
}

func TestDecodeContextStore(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectedErr bool
	}{
		{
			name:    "store with its current context",
			content: `{"contexts":{"https://okteto.example.com":{"name":"https://okteto.example.com","certificate":"cert"}},"current-context":"https://okteto.example.com"}`,
		},
		{
			name:        "store without a current context",
			content:     `{"contexts":{"https://okteto.example.com":{"name":"https://okteto.example.com","certificate":"cert"}}}`,
			expectedErr: true,
		},
		{
			name:        "current context missing from the store",
			content:     `{"contexts":{"https://okteto.example.com":{"name":"https://okteto.example.com"}},"current-context":"https://other.example.com"}`,
			expectedErr: true,
		},
		{
			name:        "field the CLI doesn't know",
			content:     `{"contexts":{"https://okteto.example.com":{"name":"https://okteto.example.com"}},"current-context":"https://okteto.example.com","new-field":true}`,
			expectedErr: true,
		},
		{
			name:        "not JSON",
			content:     "not json",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeContextStore([]byte(tt.content))

			require.Equal(t, tt.expectedErr, err != nil)
		})
	}
}

// apiRequest is what the fake Okteto API got
type apiRequest struct {
	Variables     map[string]string `json:"variables"`
	Query         string            `json:"query"`
	authorization string
	path          string
	mu            sync.Mutex
}

func (r *apiRequest) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authorization = req.Header.Get("Authorization")
	r.path = req.URL.Path
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return
	}
	_ = json.Unmarshal(body, r)
}

func fakeAPIHandler(got *apiRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got.record(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(gitCredentialsResponse))
	}
}

// useOktetoFolder points the CLI to an empty Okteto folder, so no context store is found.
// It also clears the internal server name: the override it sets is global and can't be undone.
func useOktetoFolder(t *testing.T) {
	t.Helper()
	t.Setenv(constants.OktetoFolderEnvVar, t.TempDir())
	t.Setenv(constants.OktetoInternalServerNameEnvVar, "")
	okteto.CurrentStore = nil
	t.Cleanup(func() {
		okteto.CurrentStore = nil
	})
}

// saveContextStore saves a context store with the instance and its certificate
func saveContextStore(t *testing.T, oktetoURL string, cert []byte) {
	t.Helper()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	store := okteto.ContextStore{
		CurrentContext: oktetoURL,
		Contexts: map[string]*okteto.Context{
			oktetoURL: {
				Name:        oktetoURL,
				Certificate: base64.StdEncoding.EncodeToString(certPEM),
				IsOkteto:    true,
			},
		},
	}
	content, err := json.Marshal(store)
	require.NoError(t, err)
	path := config.GetOktetoContextsStorePath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, content, 0600))
}

func TestGet_CallsTheOktetoAPIWithTheTokenInTheEnv(t *testing.T) {
	got := &apiRequest{}
	server := httptest.NewServer(fakeAPIHandler(got))
	t.Cleanup(server.Close)
	useOktetoFolder(t)
	t.Setenv(model.OktetoURLEnvVar, server.URL)
	t.Setenv(model.OktetoContextEnvVar, "")
	t.Setenv(model.OktetoTokenEnvVar, "okteto-token")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stderr)
	require.Equal(t, "username=x-access-token\npassword=token\npassword_expiry_utc=1794564000\n", stdout)
	got.mu.Lock()
	defer got.mu.Unlock()
	require.Equal(t, "Bearer okteto-token", got.authorization)
	require.Equal(t, "/graphql", got.path)
	require.Contains(t, got.Query, "gitCredentials(host: $host, repository: $repository)")
	require.Equal(t, map[string]string{"host": "github.com", "repository": "acme/infra"}, got.Variables)
}

func TestGet_TrustsTheCertificateOfTheStoredContext(t *testing.T) {
	server := httptest.NewTLSServer(fakeAPIHandler(&apiRequest{}))
	t.Cleanup(server.Close)
	useOktetoFolder(t)
	saveContextStore(t, server.URL, server.Certificate().Raw)
	t.Setenv(model.OktetoURLEnvVar, server.URL)
	t.Setenv(model.OktetoContextEnvVar, "")
	t.Setenv(model.OktetoTokenEnvVar, "okteto-token")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stderr)
	require.Equal(t, "username=x-access-token\npassword=token\npassword_expiry_utc=1794564000\n", stdout)
}

func TestGet_IgnoresAContextStoreItCantRead(t *testing.T) {
	server := httptest.NewServer(fakeAPIHandler(&apiRequest{}))
	t.Cleanup(server.Close)
	useOktetoFolder(t)
	path := config.GetOktetoContextsStorePath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0600))
	t.Setenv(model.OktetoURLEnvVar, server.URL)
	t.Setenv(model.OktetoContextEnvVar, "")
	t.Setenv(model.OktetoTokenEnvVar, "okteto-token")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stderr)
	require.Equal(t, "username=x-access-token\npassword=token\npassword_expiry_utc=1794564000\n", stdout)
}

func TestGet_FailsTLSWithoutAStoredContext(t *testing.T) {
	server := httptest.NewTLSServer(fakeAPIHandler(&apiRequest{}))
	t.Cleanup(server.Close)
	useOktetoFolder(t)
	t.Setenv(model.OktetoURLEnvVar, server.URL)
	t.Setenv(model.OktetoContextEnvVar, "")
	t.Setenv(model.OktetoTokenEnvVar, "okteto-token")

	stdout, stderr := runGet(githubRequest, newGetterFromEnv)

	require.Empty(t, stdout)
	require.Contains(t, stderr, "okteto: no GitHub credentials for acme/infra: ")
	require.Contains(t, stderr, "certificate")
}
