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

package okteto

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

const testCatalogSecretVariablesEndpoint = "https://okteto.example/internal/catalog/secret-variables"

func respondWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestGetCatalogSecretVariablesSendsTheAction(t *testing.T) {
	var got *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"variables":[]}`))
	})

	_, err := newTestUserClient(handler).GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "deploy 1")

	require.NoError(t, err)
	require.Equal(t, http.MethodGet, got.Method)
	require.Equal(t, "okteto.example", got.URL.Host)
	require.Equal(t, "/internal/catalog/secret-variables", got.URL.Path)
	require.Equal(t, "action=deploy+1", got.URL.RawQuery)
}

func TestGetCatalogSecretVariablesSendsTheToken(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"variables":[]}`))
	}))
	t.Cleanup(server.Close)
	c, err := NewOktetoClientFromUrlAndToken(server.URL, "okteto-token")
	require.NoError(t, err)

	_, err = c.User().GetCatalogSecretVariables(context.Background(), server.URL+"/internal/catalog/secret-variables", "action")

	require.NoError(t, err)
	require.Equal(t, "Bearer okteto-token", authorization)
}

func TestGetCatalogSecretVariablesReturnsTheValues(t *testing.T) {
	handler := respondWith(http.StatusOK, `{"variables":[{"name":"API_TOKEN","value":"abc"},{"name":"PIN","value":"1"}],"unresolved":[]}`)

	got, err := newTestUserClient(handler).GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "action")

	require.NoError(t, err)
	require.Equal(t, []types.CatalogSecretVariable{
		{Name: "API_TOKEN", Value: "abc"},
		{Name: "PIN", Value: "1"},
	}, got.Variables)
	require.Empty(t, got.Unresolved)
}

func TestGetCatalogSecretVariablesWithoutVariables(t *testing.T) {
	handler := respondWith(http.StatusOK, `{"variables":[],"unresolved":[]}`)

	got, err := newTestUserClient(handler).GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "action")

	require.NoError(t, err)
	require.Empty(t, got.Variables)
	require.Empty(t, got.Unresolved)
}

func TestGetCatalogSecretVariablesReturnsTheUnresolvedVariables(t *testing.T) {
	handler := respondWith(http.StatusOK, `{"variables":[{"name":"PIN","value":"1"}],"unresolved":[
		{"variable":"API_TOKEN","secretKeyRef":{"name":"creds","key":"token"},"reason":"secret_not_found"},
		{"variable":"DB_PASSWORD","secretKeyRef":{"name":"db","key":"password"},"reason":"key_not_found"},
		{"variable":"NPM_TOKEN","secretKeyRef":{"name":"npm","key":"token"},"reason":"secret_not_labeled"}
	]}`)

	got, err := newTestUserClient(handler).GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "action")

	require.NoError(t, err)
	require.Equal(t, []types.CatalogSecretVariable{{Name: "PIN", Value: "1"}}, got.Variables)
	require.Equal(t, []types.UnresolvedCatalogSecretVariable{
		{Variable: "API_TOKEN", SecretKeyRef: types.CatalogSecretKeyRef{Name: "creds", Key: "token"}, Reason: types.CatalogSecretNotFoundReason},
		{Variable: "DB_PASSWORD", SecretKeyRef: types.CatalogSecretKeyRef{Name: "db", Key: "password"}, Reason: types.CatalogSecretKeyNotFoundReason},
		{Variable: "NPM_TOKEN", SecretKeyRef: types.CatalogSecretKeyRef{Name: "npm", Key: "token"}, Reason: types.CatalogSecretNotLabeledReason},
	}, got.Unresolved)
}

func TestGetCatalogSecretVariablesWithFeatureDisabled(t *testing.T) {
	handler := respondWith(http.StatusUnprocessableEntity, `{"reason":"feature_disabled"}`)

	_, err := newTestUserClient(handler).GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "action")

	require.ErrorIs(t, err, ErrCatalogSecretVariablesDisabled)
}

func TestGetCatalogSecretVariablesFailures(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
		status   int
	}{
		{name: "no action", status: http.StatusBadRequest, expected: "catalog secret variables request failed: 400 Bad Request"},
		{name: "unauthorized", status: http.StatusUnauthorized, expected: "catalog secret variables request failed: 401 Unauthorized"},
		{name: "not found", status: http.StatusNotFound, expected: "catalog secret variables request failed: 404 Not Found"},
		{name: "server error", status: http.StatusInternalServerError, expected: "catalog secret variables request failed: 500 Internal Server Error"},
		{name: "422 without a body", status: http.StatusUnprocessableEntity, expected: "catalog secret variables request failed: 422 Unprocessable Entity"},
		{name: "422 with an unknown reason", status: http.StatusUnprocessableEntity, body: `{"reason":"other"}`, expected: "catalog secret variables request failed: 422 Unprocessable Entity"},
		{name: "invalid 200 body", status: http.StatusOK, body: `{"variables":`, expected: "failed to read the catalog secret variables"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := respondWith(tt.status, tt.body)

			got, err := newTestUserClient(handler).GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "action")

			require.ErrorContains(t, err, tt.expected)
			require.Empty(t, got)
		})
	}
}

func TestGetCatalogSecretVariablesUnreachable(t *testing.T) {
	c := &userClient{
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			}),
		},
	}

	_, err := c.GetCatalogSecretVariables(context.Background(), testCatalogSecretVariablesEndpoint, "action")

	require.ErrorContains(t, err, "could not reach the catalog secret variables endpoint")
	require.ErrorContains(t, err, "connection refused")
}

func TestGetCatalogSecretVariablesWithInvalidEndpoint(t *testing.T) {
	_, err := newTestUserClient(respondWith(http.StatusOK, "")).GetCatalogSecretVariables(context.Background(), "http://[::1", "action")

	require.ErrorContains(t, err, "invalid catalog secret variables endpoint")
}
