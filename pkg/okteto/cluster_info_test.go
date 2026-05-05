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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetClusterInfo(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.Handler
		expected    *types.ClusterInfo
		expectedErr string
	}{
		{
			name: "success response",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/clusterinfo", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(types.ClusterInfo{
					ClusterVersion: "1.2.3",
					CustomerName:   "ACME Corp",
				})
			}),
			expected: &types.ClusterInfo{
				ClusterVersion: "1.2.3",
				CustomerName:   "ACME Corp",
			},
		},
		{
			name: "non success response",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			expectedErr: "clusterinfo request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{
				Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
					recorder := &responseRecorder{
						header: http.Header{},
						body:   &bytes.Buffer{},
					}
					tt.handler.ServeHTTP(recorder, r)
					return recorder.Response(), nil
				}),
			}

			c := &userClient{
				httpClient: httpClient,
				baseURL:    "https://okteto.example.com",
			}

			got, err := c.GetClusterInfo(context.Background())
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

type responseRecorder struct {
	code   int
	header http.Header
	body   *bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.code = statusCode
}

func (r *responseRecorder) Response() *http.Response {
	statusCode := r.code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     r.header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(r.body.Bytes())),
	}
}
