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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func Test_IsLocalHostname(t *testing.T) {
	var tests = []struct {
		name     string
		hostname string
		expected bool
	}{
		{
			name:     "172 non rfc",
			hostname: "https://172.15.1.2:16443",
			expected: false,
		},
		{
			name:     "192 non rfc",
			hostname: "https://192.15.1.2:16443",
			expected: false,
		},
		{
			name:     "169 no unicast",
			hostname: "https://169.15.1.2:16443",
			expected: false,
		},
		{
			name:     "microk8s",
			hostname: "https://172.16.29.3:16443",
			expected: true,
		},
		{
			name:     "minikube",
			hostname: "https://172.16.29.2:8443",
			expected: true,
		},
		{
			name:     "localhost",
			hostname: "https://127.0.0.1",
			expected: true,
		},
		{
			name:     "localhost-ipv6",
			hostname: "::1",
			expected: true,
		},
		{
			name:     "local-2",
			hostname: "https://192.168.1.24:123",
			expected: true,
		},
		{
			name:     "local other computer",
			hostname: "https://169.254.1.2:16443",
			expected: true,
		},
		{
			name:     "k3d",
			hostname: "https://0.0.0.0",
			expected: true,
		},
		{
			name:     "docker",
			hostname: "https://kubernetes.docker.internal:123",
			expected: true,
		},
		{
			name:     "localhost-ipv6-unicast",
			hostname: "fe80::9656:d028:8652:66b6",
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isLocalHostname(tt.hostname) != tt.expected {
				t.Fatal("not correct")
			}
		})
	}
}

func TestKubetokenRefreshRoundTrip(t *testing.T) {
	tt := []struct {
		handleFunc   http.HandlerFunc
		expectedErr  error
		name         string
		expectedCode int
	}{
		{
			name: "ok",
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Test Response"))
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "not found",
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("Test Response"))
			},
			expectedCode: http.StatusNotFound,
		},
		{
			name: "unauthorized",
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Test Response"))
			},
			expectedErr: ErrK8sUnauthorised,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			testServer := httptest.NewServer(tc.handleFunc)
			defer testServer.Close()
			transport := newTokenRotationTransport(http.DefaultTransport, io.NewK8sLogger())
			client := &http.Client{
				Transport: transport,
			}
			req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.ErrorIs(t, err, tc.expectedErr)
			if resp != nil {
				require.NoError(t, resp.Body.Close())
				require.Equal(t, tc.expectedCode, resp.StatusCode)
			}
		})
	}
}

func TestGetK8sClientWithApiConfig(t *testing.T) {
	type expected struct {
		err error
		cfg *rest.Config
	}
	tt := []struct {
		expected  expected
		apiConfig *clientcmdapi.Config
		name      string
	}{
		{
			name: "ok",
			apiConfig: &clientcmdapi.Config{
				Clusters: map[string]*clientcmdapi.Cluster{
					"test": {
						Server: "https://test.com",
					},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"test": {
						Token: "test",
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"test": {
						Cluster:  "test",
						AuthInfo: "test",
					},
				},
				CurrentContext: "test",
			},
			expected: expected{
				cfg: &rest.Config{
					Host: "https://test.com",
					TLSClientConfig: rest.TLSClientConfig{
						Insecure: true,
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			client, cfg, err := getK8sClientWithApiConfig(tc.apiConfig, nil)
			require.ErrorIs(t, err, tc.expected.err)
			require.NotNil(t, client)
			require.NotNil(t, cfg)
			require.Equal(t, tc.expected.cfg.Host, cfg.Host)
			require.NotNil(t, cfg.WrapTransport)
			require.Equal(t, cfg.WarningHandler, rest.NoWarnings{})
		})
	}

}
