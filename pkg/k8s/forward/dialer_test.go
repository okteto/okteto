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

package forward

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/streaming/pkg/httpstream"
)

type upgradeRequest struct {
	method  string
	upgrade string
}

// upgradeRecorder is a fake API server that rejects every upgrade request and records it
type upgradeRecorder struct {
	server   *httptest.Server
	requests []upgradeRequest
	mu       sync.Mutex
}

func newUpgradeRecorder(t *testing.T) *upgradeRecorder {
	t.Helper()
	r := &upgradeRecorder{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests = append(r.requests, upgradeRequest{method: req.Method, upgrade: req.Header.Get(httpstream.HeaderUpgrade)})
		r.mu.Unlock()
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *upgradeRecorder) dial(t *testing.T) error {
	t.Helper()
	u, err := url.Parse(r.server.URL + "/api/v1/namespaces/test/pods/test/portforward")
	require.NoError(t, err)

	dialer, err := NewDialer(&rest.Config{Host: r.server.URL}, u)
	require.NoError(t, err)

	_, _, err = dialer.Dial(portforward.PortForwardProtocolV1Name)
	return err
}

func (r *upgradeRecorder) recorded() []upgradeRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]upgradeRequest(nil), r.requests...)
}

func TestNewDialerFallsBackFromWebSocketsToSPDY(t *testing.T) {
	t.Setenv(string(cmdutil.PortForwardWebsockets), "")
	r := newUpgradeRecorder(t)

	require.Error(t, r.dial(t))
	require.Equal(t, []upgradeRequest{
		{method: http.MethodGet, upgrade: "websocket"},
		{method: http.MethodPost, upgrade: "SPDY/3.1"},
	}, r.recorded())
}

func TestNewDialerOnlyUsesSPDYWhenWebSocketsAreDisabled(t *testing.T) {
	t.Setenv(string(cmdutil.PortForwardWebsockets), "false")
	r := newUpgradeRecorder(t)

	require.Error(t, r.dial(t))
	require.Equal(t, []upgradeRequest{
		{method: http.MethodPost, upgrade: "SPDY/3.1"},
	}, r.recorded())
}

func TestNewDialerReturnsErrorWithInvalidTLSConfig(t *testing.T) {
	restConfig := &rest.Config{
		Host:            "https://localhost",
		TLSClientConfig: rest.TLSClientConfig{CAFile: "/non-existent/ca.crt"},
	}

	_, err := NewDialer(restConfig, &url.URL{})
	require.Error(t, err)
}

func TestShouldFallbackToSPDY(t *testing.T) {
	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "upgrade failure",
			err:      &httpstream.UpgradeFailureError{Cause: errors.New("bad handshake")},
			expected: true,
		},
		{
			name:     "https proxy error",
			err:      errors.New("proxy: unknown scheme: https"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, shouldFallbackToSPDY(tt.err))
		})
	}
}
