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
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestUserClient(handler http.Handler) *userClient {
	return &userClient{
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				recorder := &responseRecorder{
					header: http.Header{},
					body:   &bytes.Buffer{},
				}
				handler.ServeHTTP(recorder, r)
				return recorder.Response(), nil
			}),
		},
		baseURL: "https://okteto.example.com",
	}
}

func TestGetClusterInfo_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/clusterinfo", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(types.ClusterInfo{
			ClusterVersion: "1.2.3",
			CustomerName:   "ACME Corp",
		})
	})

	got, err := newTestUserClient(handler).GetClusterInfo(context.Background())

	require.NoError(t, err)
	require.Equal(t, &types.ClusterInfo{
		ClusterVersion: "1.2.3",
		CustomerName:   "ACME Corp",
	}, got)
}

func TestGetClusterInfo_NonSuccessResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := newTestUserClient(handler).GetClusterInfo(context.Background())

	require.ErrorContains(t, err, "clusterinfo request failed")
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
