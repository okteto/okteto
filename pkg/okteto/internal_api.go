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
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"

	oktetoHttp "github.com/okteto/okteto/pkg/http"
)

var (
	// ErrInvalidInternalAPIEndpoint is returned for an internal API endpoint that isn't an https URL
	ErrInvalidInternalAPIEndpoint = errors.New("the internal API endpoint isn't an https URL")

	// ErrInvalidInternalAPIIP is returned when the internal IP of the Okteto API is empty or isn't an IP
	ErrInvalidInternalAPIIP = errors.New("the internal IP of the Okteto API is missing or invalid")
)

// NewInternalAPIHTTPClient returns an HTTP client for the internal routes of the Okteto API at endpoint, an https URL
// that remote builds reach on internalAPIIP, the internal IP of the API. It connects to that IP instead of resolving
// the hostname of the endpoint, which public DNS resolves to the ingress, and verifies the certificate against that
// hostname, with the system pool and certs. Without a valid IP it fails instead of resolving the hostname.
//
// It never goes through a proxy, as the IP is only reachable inside the cluster, and never follows a redirect, which
// would take the request somewhere else. It doesn't read the Okteto context: the Git credential helper runs without one
func NewInternalAPIHTTPClient(endpoint, internalAPIIP string, certs ...*x509.Certificate) (*http.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, ErrInvalidInternalAPIEndpoint
	}
	ip := net.ParseIP(internalAPIIP)
	if ip == nil {
		return nil, ErrInvalidInternalAPIIP
	}

	transport := oktetoHttp.StrictSSLTransport(&oktetoHttp.SSLTransportOption{
		Certs:           certs,
		URLsToIntercept: []string{endpoint},
		ServerName:      internalAPIAddress(u, ip),
	})
	transport.Proxy = nil
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// internalAPIAddress returns the address to connect to for the endpoint: the internal IP with the port of the endpoint
func internalAPIAddress(endpoint *url.URL, ip net.IP) string {
	port := endpoint.Port()
	if port == "" {
		port = "443"
	}
	return net.JoinHostPort(ip.String(), port)
}
