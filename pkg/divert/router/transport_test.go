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

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// markerTransport records that it was the one chosen.
type markerTransport struct {
	used bool
}

func (t *markerTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.used = true

	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func newMarkedTransport() (*transport, *markerTransport, *markerTransport) {
	http1, http2 := &markerTransport{}, &markerTransport{}

	return &transport{http1: http1, http2: http2}, http1, http2
}

// gRPC is HTTP/2 over cleartext. Sending it over the HTTP/1.1 transport is a silent
// downgrade that makes it hang rather than fail.
func TestTransport_SendsHTTP2OverTheH2CTransport(t *testing.T) {
	tr, http1, http2 := newMarkedTransport()
	req := httptest.NewRequest(http.MethodPost, testRequestURL, nil)
	req.ProtoMajor = 2

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.True(t, http2.used)
	require.False(t, http1.used)
}

// HTTP/1.1 must keep the standard transport: it is the only one that can carry a
// `Connection: Upgrade` exchange, which HTTP/2 has no equivalent for.
func TestTransport_SendsHTTP1OverTheStandardTransport(t *testing.T) {
	tr, http1, http2 := newMarkedTransport()
	req := httptest.NewRequest(http.MethodGet, testRequestURL, nil)
	req.ProtoMajor = 1

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.True(t, http1.used)
	require.False(t, http2.used)
}

func TestNewTransport_TalksCleartextHTTP2(t *testing.T) {
	tr, ok := newTransport().(*transport)
	require.True(t, ok)

	h2, ok := tr.http2.(*http2.Transport)
	require.True(t, ok)

	// Nothing in the cluster speaks TLS to the router, so there is no ALPN to negotiate:
	// AllowHTTP plus a plain dialer is what makes prior-knowledge h2c work.
	require.True(t, h2.AllowHTTP)
	require.NotNil(t, h2.DialTLSContext)
}
