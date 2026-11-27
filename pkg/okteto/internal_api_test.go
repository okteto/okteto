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

// endpoint returns the endpoint of the server with host, a hostname or an IP
func (s *internalAPITestServer) endpoint(t *testing.T, host string) string {
	t.Helper()
	return fmt.Sprintf("https://%s/internal/catalog/secret-variables", net.JoinHostPort(host, s.port(t)))
}

func (s *internalAPITestServer) port(t *testing.T) string {
	t.Helper()
	_, port, err := net.SplitHostPort(s.Listener.Addr().String())
	require.NoError(t, err)
	return port
}

func answerOK(http.ResponseWriter, *http.Request) {}

// The endpoint has a hostname that doesn't resolve to the server, so the
// requests only reach it through the IP
func TestInternalAPIHTTPClientConnectsToTheIPForAnEndpointWithTheHostname(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{name: "IPv4", ip: "127.0.0.1"},
		{name: "IPv6", ip: "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := startInternalAPITestServer(t, tt.ip, answerOK)
			endpoint := server.endpoint(t, "example.com")
			client, err := NewInternalAPIHTTPClient(endpoint, InternalAPI{IP: tt.ip}, server.Certificate())
			require.NoError(t, err)

			resp, err := client.Get(endpoint)

			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "example.com", server.sni)
			require.Equal(t, "example.com:"+server.port(t), server.host)
		})
	}
}

// The request is the same as for an endpoint with the hostname: the hostname
// is the SNI and the Host
func TestInternalAPIHTTPClientConnectsToTheIPOfAnEndpointWithTheIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{name: "IPv4", ip: "127.0.0.1"},
		{name: "IPv6", ip: "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := startInternalAPITestServer(t, tt.ip, answerOK)
			endpoint := server.endpoint(t, tt.ip)
			client, err := NewInternalAPIHTTPClient(endpoint, InternalAPI{Hostname: "example.com"}, server.Certificate())
			require.NoError(t, err)

			resp, err := client.Get(endpoint)

			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "example.com", server.sni)
			require.Equal(t, "example.com:"+server.port(t), server.host)
		})
	}
}

// The certificate is also valid for 127.0.0.1, so verifying it against the IP
// would let the request through
func TestInternalAPIHTTPClientVerifiesTheCertificateAgainstTheHostname(t *testing.T) {
	tests := []struct {
		name         string
		endpointHost string
		api          InternalAPI
	}{
		{
			name:         "endpoint with the hostname",
			endpointHost: "okteto-internal-api.other.example",
			api:          InternalAPI{IP: "127.0.0.1"},
		},
		{
			name:         "endpoint with the IP",
			endpointHost: "127.0.0.1",
			api:          InternalAPI{Hostname: "okteto.other.example"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := startInternalAPITestServer(t, "127.0.0.1", answerOK)
			endpoint := server.endpoint(t, tt.endpointHost)
			client, err := NewInternalAPIHTTPClient(endpoint, tt.api, server.Certificate())
			require.NoError(t, err)

			_, err = client.Get(endpoint) //nolint:bodyclose // it fails before there's a body

			require.ErrorContains(t, err, "certificate is valid for")
			require.Empty(t, server.sni)
		})
	}
}

func TestInternalAPIHTTPClientDoesNotTrustTheCertificateWithoutIt(t *testing.T) {
	server := startInternalAPITestServer(t, "127.0.0.1", answerOK)
	endpoint := server.endpoint(t, "127.0.0.1")
	client, err := NewInternalAPIHTTPClient(endpoint, InternalAPI{Hostname: "example.com"})
	require.NoError(t, err)

	_, err = client.Get(endpoint) //nolint:bodyclose // it fails before there's a body

	require.ErrorContains(t, err, "certificate signed by unknown authority")
}

// The insecure mode of the CLI skips the certificate check, as it does for the
// Okteto API
func TestInternalAPIHTTPClientSkipsTheCertificateCheckInInsecureMode(t *testing.T) {
	original := insecureSkipTLSVerify
	insecureSkipTLSVerify = true
	t.Cleanup(func() { insecureSkipTLSVerify = original })
	server := startInternalAPITestServer(t, "127.0.0.1", answerOK)
	endpoint := server.endpoint(t, "127.0.0.1")
	client, err := NewInternalAPIHTTPClient(endpoint, InternalAPI{Hostname: "okteto.other.example"})
	require.NoError(t, err)

	resp, err := client.Get(endpoint)

	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "okteto.other.example", server.sni)
}

func TestInternalAPIHTTPClientDoesNotFollowRedirects(t *testing.T) {
	server := startInternalAPITestServer(t, "127.0.0.1", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://okteto.example/elsewhere", http.StatusFound)
	})
	endpoint := server.endpoint(t, "127.0.0.1")
	client, err := NewInternalAPIHTTPClient(endpoint, InternalAPI{Hostname: "example.com"}, server.Certificate())
	require.NoError(t, err)

	resp, err := client.Get(endpoint)

	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusFound, resp.StatusCode)
}

func TestInternalAPIHTTPClientNeverUsesAProxy(t *testing.T) {
	client, err := NewInternalAPIHTTPClient("https://10.20.30.40:8443/internal/catalog/secret-variables", InternalAPI{Hostname: "example.com"})
	require.NoError(t, err)

	transport, isTransport := client.Transport.(*internalAPITransport).base.(*http.Transport)

	require.True(t, isTransport)
	require.Nil(t, transport.Proxy)
}

func TestNewInternalAPITarget(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		api      InternalAPI
		expected internalAPITarget
	}{
		{
			name:     "hostname with IPv4",
			endpoint: "https://example.com:8443/",
			api:      InternalAPI{IP: "10.20.30.40"},
			expected: internalAPITarget{address: "10.20.30.40:8443", hostname: "example.com", host: "example.com:8443"},
		},
		{
			name:     "hostname with IPv6",
			endpoint: "https://example.com:8443/",
			api:      InternalAPI{IP: "fd00:0::1"},
			expected: internalAPITarget{address: "[fd00::1]:8443", hostname: "example.com", host: "example.com:8443"},
		},
		{
			name:     "hostname without port",
			endpoint: "https://example.com/",
			api:      InternalAPI{IP: "10.20.30.40"},
			expected: internalAPITarget{address: "10.20.30.40:443", hostname: "example.com", host: "example.com"},
		},
		{
			name:     "hostname ignores the hostname of api",
			endpoint: "https://example.com:8443/",
			api:      InternalAPI{Hostname: "okteto.example.com", IP: "10.20.30.40"},
			expected: internalAPITarget{address: "10.20.30.40:8443", hostname: "example.com", host: "example.com:8443"},
		},
		{
			name:     "IPv4",
			endpoint: "https://10.20.30.40:8443/",
			api:      InternalAPI{Hostname: "okteto.example.com"},
			expected: internalAPITarget{address: "10.20.30.40:8443", hostname: "okteto.example.com", host: "okteto.example.com:8443"},
		},
		{
			name:     "IPv6",
			endpoint: "https://[fd00:0::1]:8443/",
			api:      InternalAPI{Hostname: "okteto.example.com"},
			expected: internalAPITarget{address: "[fd00::1]:8443", hostname: "okteto.example.com", host: "okteto.example.com:8443"},
		},
		{
			name:     "IP without port",
			endpoint: "https://10.20.30.40/",
			api:      InternalAPI{Hostname: "okteto.example.com"},
			expected: internalAPITarget{address: "10.20.30.40:443", hostname: "okteto.example.com", host: "okteto.example.com"},
		},
		{
			name:     "IP ignores the IP of api",
			endpoint: "https://10.20.30.40:8443/",
			api:      InternalAPI{Hostname: "okteto.example.com", IP: "10.0.0.1"},
			expected: internalAPITarget{address: "10.20.30.40:8443", hostname: "okteto.example.com", host: "okteto.example.com:8443"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newInternalAPITarget(tt.endpoint, tt.api)

			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

// An endpoint with a hostname needs the IP even with the hostname, as the
// client never resolves it
func TestInternalAPIHTTPClientFailsForAnEndpointWithTheHostnameWithoutAValidIP(t *testing.T) {
	tests := []struct {
		name string
		api  InternalAPI
	}{
		{name: "without IP", api: InternalAPI{}},
		{name: "with the hostname", api: InternalAPI{Hostname: "example.com"}},
		{name: "IPv6 in brackets", api: InternalAPI{IP: "[fd00::1]"}},
		{name: "hostname", api: InternalAPI{IP: "okteto-api.okteto"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInternalAPIHTTPClient("https://example.com:8443/internal/catalog/secret-variables", tt.api)

			require.ErrorIs(t, err, ErrInvalidInternalAPIIP)
		})
	}
}

func TestInternalAPIHTTPClientFailsForAnEndpointWithTheIPWithoutAValidHostname(t *testing.T) {
	tests := []struct {
		name string
		api  InternalAPI
	}{
		{name: "without hostname", api: InternalAPI{}},
		{name: "with the IP", api: InternalAPI{IP: "10.20.30.40"}},
		{name: "with a port", api: InternalAPI{Hostname: "okteto.example.com:8443"}},
		{name: "URL", api: InternalAPI{Hostname: "https://okteto.example.com"}},
		{name: "with a new line", api: InternalAPI{Hostname: "okteto.example.com\nRUN id"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInternalAPIHTTPClient("https://10.20.30.40:8443/internal/catalog/secret-variables", tt.api)

			require.ErrorIs(t, err, ErrInvalidInternalAPIHostname)
		})
	}
}

func TestInternalAPIHTTPClientFailsWithoutAnHTTPSEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "http", endpoint: "http://10.20.30.40:8080/internal/catalog/secret-variables"},
		{name: "without host", endpoint: "https:///internal/catalog/secret-variables"},
		{name: "invalid", endpoint: "https://10.20.30.40:port/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInternalAPIHTTPClient(tt.endpoint, InternalAPI{Hostname: "example.com", IP: "10.20.30.40"})

			require.ErrorIs(t, err, ErrInvalidInternalAPIEndpoint)
		})
	}
}

func TestCheckInternalAPIEndpoint(t *testing.T) {
	tests := []struct {
		expected error
		name     string
		endpoint string
		api      InternalAPI
	}{
		{name: "endpoint with the IP", endpoint: "https://10.20.30.40:8443/", api: InternalAPI{Hostname: "example.com"}},
		{name: "endpoint with the hostname", endpoint: "https://example.com:8443/", api: InternalAPI{IP: "10.20.30.40"}},
		{name: "endpoint with the IP without hostname", endpoint: "https://10.20.30.40:8443/", api: InternalAPI{IP: "10.20.30.40"}, expected: ErrInvalidInternalAPIHostname},
		{name: "endpoint with the hostname without IP", endpoint: "https://example.com:8443/", api: InternalAPI{Hostname: "example.com"}, expected: ErrInvalidInternalAPIIP},
		{name: "http endpoint", endpoint: "http://10.20.30.40:8443/", api: InternalAPI{Hostname: "example.com"}, expected: ErrInvalidInternalAPIEndpoint},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, CheckInternalAPIEndpoint(tt.endpoint, tt.api), tt.expected)
		})
	}
}

func TestInternalAPIHostname(t *testing.T) {
	tests := []struct {
		name        string
		contextName string
		expected    string
	}{
		{name: "URL", contextName: "https://okteto.example.com", expected: "okteto.example.com"},
		{name: "URL with a port", contextName: "https://okteto.example.com:8443", expected: "okteto.example.com"},
		{name: "URL with a path", contextName: "https://okteto.example.com/", expected: "okteto.example.com"},
		{name: "without scheme", contextName: "okteto.example.com", expected: "okteto.example.com"},
		{name: "uppercase", contextName: "https://Okteto.Example.com", expected: "okteto.example.com"},
		{name: "empty", contextName: "", expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, InternalAPIHostname(tt.contextName))
		})
	}
}
