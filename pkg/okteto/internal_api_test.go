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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// internalAPITestServer is a TLS server whose certificate is valid for
// example.com, 127.0.0.1 and ::1, and which records the Host header and the SNI
// of the requests
type internalAPITestServer struct {
	*httptest.Server
	host string
	sni  string
}

// startInternalAPITestServer starts the server on the IP. It skips the test
// when it can't listen on the IP
func startInternalAPITestServer(t *testing.T, ip string, handler http.HandlerFunc) *internalAPITestServer {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Skipf("can't listen on %s: %s", ip, err)
	}
	s := &internalAPITestServer{}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.host = r.Host
		s.sni = r.TLS.ServerName
		handler(w, r)
	}))
	s.Listener = listener
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

func (s *internalAPITestServer) endpoint(t *testing.T, hostname string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(s.Listener.Addr().String())
	require.NoError(t, err)
	return fmt.Sprintf("https://%s/internal/catalog/secret-variables", net.JoinHostPort(hostname, port))
}

func answerOK(http.ResponseWriter, *http.Request) {}

func TestInternalAPIHTTPClientConnectsToTheIPv4(t *testing.T) {
	server := startInternalAPITestServer(t, "127.0.0.1", answerOK)
	endpoint := server.endpoint(t, "example.com")
	client, err := NewInternalAPIHTTPClient(endpoint, "127.0.0.1", server.Certificate())
	require.NoError(t, err)

	resp, err := client.Get(endpoint)

	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "example.com", server.sni)
	require.Equal(t, mustURL(t, endpoint).Host, server.host)
}

func TestInternalAPIHTTPClientConnectsToTheIPv6(t *testing.T) {
	server := startInternalAPITestServer(t, "::1", answerOK)
	endpoint := server.endpoint(t, "example.com")
	client, err := NewInternalAPIHTTPClient(endpoint, "::1", server.Certificate())
	require.NoError(t, err)

	resp, err := client.Get(endpoint)

	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "example.com", server.sni)
	require.Equal(t, mustURL(t, endpoint).Host, server.host)
}

func TestInternalAPIHTTPClientVerifiesTheCertificateAgainstTheHostname(t *testing.T) {
	// The certificate is also valid for 127.0.0.1, so verifying it against the
	// IP would let the request through
	server := startInternalAPITestServer(t, "127.0.0.1", answerOK)
	endpoint := server.endpoint(t, "okteto-internal-api.other.example")
	client, err := NewInternalAPIHTTPClient(endpoint, "127.0.0.1", server.Certificate())
	require.NoError(t, err)

	_, err = client.Get(endpoint) //nolint:bodyclose // it fails before there's a body

	require.ErrorContains(t, err, "certificate is valid for")
	require.Empty(t, server.sni)
}

func TestInternalAPIHTTPClientDoesNotTrustTheCertificateWithoutIt(t *testing.T) {
	server := startInternalAPITestServer(t, "127.0.0.1", answerOK)
	endpoint := server.endpoint(t, "example.com")
	client, err := NewInternalAPIHTTPClient(endpoint, "127.0.0.1")
	require.NoError(t, err)

	_, err = client.Get(endpoint) //nolint:bodyclose // it fails before there's a body

	require.ErrorContains(t, err, "certificate signed by unknown authority")
}

func TestInternalAPIHTTPClientDoesNotFollowRedirects(t *testing.T) {
	server := startInternalAPITestServer(t, "127.0.0.1", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://okteto.example/elsewhere", http.StatusFound)
	})
	endpoint := server.endpoint(t, "example.com")
	client, err := NewInternalAPIHTTPClient(endpoint, "127.0.0.1", server.Certificate())
	require.NoError(t, err)

	resp, err := client.Get(endpoint)

	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusFound, resp.StatusCode)
}

func TestInternalAPIHTTPClientNeverUsesAProxy(t *testing.T) {
	client, err := NewInternalAPIHTTPClient("https://example.com:8443/internal/catalog/secret-variables", "10.20.30.40")
	require.NoError(t, err)

	transport, isTransport := client.Transport.(*http.Transport)

	require.True(t, isTransport)
	require.Nil(t, transport.Proxy)
}

func TestInternalAPIHTTPClientFailsWithoutAValidIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{name: "without IP", ip: ""},
		{name: "IPv6 in brackets", ip: "[fd00::1]"},
		{name: "hostname", ip: "okteto-api.okteto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInternalAPIHTTPClient("https://example.com:8443/internal/catalog/secret-variables", tt.ip)

			require.ErrorIs(t, err, ErrInvalidInternalAPIIP)
		})
	}
}

func TestInternalAPIHTTPClientFailsWithoutAnHTTPSEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "http", endpoint: "http://okteto-api.okteto:8080/internal/catalog/secret-variables"},
		{name: "without hostname", endpoint: "https:///internal/catalog/secret-variables"},
		{name: "invalid", endpoint: "https://example.com:port/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInternalAPIHTTPClient(tt.endpoint, "10.20.30.40")

			require.ErrorIs(t, err, ErrInvalidInternalAPIEndpoint)
		})
	}
}

func TestInternalAPIAddress(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		ip       string
		expected string
	}{
		{name: "IPv4", endpoint: "https://example.com:8443/", ip: "10.20.30.40", expected: "10.20.30.40:8443"},
		{name: "IPv6", endpoint: "https://example.com:8443/", ip: "fd00:0::1", expected: "[fd00::1]:8443"},
		{name: "without port", endpoint: "https://example.com/", ip: "10.20.30.40", expected: "10.20.30.40:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, internalAPIAddress(mustURL(t, tt.endpoint), net.ParseIP(tt.ip)))
		})
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
