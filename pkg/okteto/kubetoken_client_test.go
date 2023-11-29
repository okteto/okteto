// Copyright 2023 The Okteto Authors
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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
)

func Test_GetKubeToken(t *testing.T) {
	tests := []struct {
		httpFakeHandler http.Handler
		expectedErr     error
		name            string
		namespace       string
		expectedToken   string
	}{
		{
			name: "error request status unauthorized",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			}),
			expectedErr: errUnauthorized,
		},
		{
			name: "error request not success",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			expectedErr: errStatus,
		},
		{
			name: "success response",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				mockResponse := types.KubeTokenResponse{
					TokenRequest: authenticationv1.TokenRequest{
						Status: authenticationv1.TokenRequestStatus{
							Token: "token",
						},
					},
				}
				jsonBytes, _ := json.Marshal(mockResponse)
				w.Write(jsonBytes)
			}),
			expectedToken: "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHttpServer := httptest.NewServer(tt.httpFakeHandler)
			defer fakeHttpServer.Close()

			fakeKubetokenClient := &kubeTokenClient{
				httpClient: fakeHttpServer.Client(),
			}

			got, err := fakeKubetokenClient.GetKubeToken(fakeHttpServer.URL, tt.namespace)
			if tt.expectedToken != "" {
				assert.Equal(t, tt.expectedToken, got.Status.Token)
			}
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func Test_CheckService(t *testing.T) {
	tests := []struct {
		httpFakeHandler http.Handler
		expectedErr     error
		name            string
		namespace       string
	}{
		{
			name: "error request status unauthorized",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			}),
			expectedErr: errKubetokenNotAvailable,
		},
		{
			name: "error service not available",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			expectedErr: errKubetokenNotAvailable,
		},
		{
			name: "success response",
			httpFakeHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHttpServer := httptest.NewServer(tt.httpFakeHandler)
			defer fakeHttpServer.Close()

			fakeKubetokenClient := &kubeTokenClient{
				httpClient: fakeHttpServer.Client(),
			}

			err := fakeKubetokenClient.CheckService(fakeHttpServer.URL, tt.namespace)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
