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
	"strings"

	oktetoHttp "github.com/okteto/okteto/pkg/http"
	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	// ErrInvalidInternalAPIEndpoint is returned for an internal API endpoint that isn't an https URL
	ErrInvalidInternalAPIEndpoint = errors.New("the internal API endpoint isn't an https URL")

	// ErrInvalidInternalAPIIP is returned for an endpoint with a hostname when the internal IP of the Okteto API is
	// empty or isn't an IP
	ErrInvalidInternalAPIIP = errors.New("the internal IP of the Okteto API is missing or invalid")

	// ErrInvalidInternalAPIHostname is returned for an endpoint with an IP when the hostname of the internal API is
	// empty or isn't a hostname
	ErrInvalidInternalAPIHostname = errors.New("the hostname of the internal API is missing or invalid")
)

// InternalAPI is what a client needs, besides the endpoint, to reach the internal API of Okteto. An endpoint has the
// internal IP of the Okteto API or the hostname of its certificate as its host, and the client takes the other one
// from here: Hostname for an endpoint with the IP, IP for one with the hostname
type InternalAPI struct {
	// Hostname is the hostname the certificate of the internal API is valid for
	Hostname string
	// IP is the internal IP of the Okteto API
	IP string
}

// InternalAPIHostname returns the hostname of the Okteto context named contextName, the URL of the Okteto API, in
// lowercase and without the scheme or the port. The internal API serves the certificate of the Okteto API, so the CLI
// checks it against the same hostname. It's empty when contextName has no host
func InternalAPIHostname(contextName string) string {
	if !strings.Contains(contextName, "://") {
		contextName = "https://" + contextName
	}
	u, err := url.Parse(contextName)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// isValidInternalAPIHostname is true when hostname can be the hostname of the internal API
func isValidInternalAPIHostname(hostname string) bool {
	return len(validation.IsDNS1123Subdomain(hostname)) == 0
}

// CheckInternalAPIEndpoint returns the error NewInternalAPIHTTPClient would return for the endpoint and api, if any
func CheckInternalAPIEndpoint(endpoint string, api InternalAPI) error {
	_, err := newInternalAPITarget(endpoint, api)
	return err
}

// NewInternalAPIHTTPClient returns an HTTP client for the internal routes of the Okteto API at endpoint, an https URL
// that remote builds reach on the internal IP of the API. The host of the endpoint is that IP, and then api has the
// hostname, or the hostname, and then api has the IP. Either way, it connects to the IP, sends the hostname as the
// Host and verifies the certificate against it, with the system pool and certs. It never resolves the hostname, which
// public DNS resolves to the ingress, and fails instead when api lacks what the endpoint needs.
//
// With the insecure mode of the CLI on, it skips the certificate check, as the client of the Okteto API does. It never
// goes through a proxy, as the IP is only reachable inside the cluster, and never follows a redirect, which would take
// the request somewhere else. It doesn't read the Okteto context
func NewInternalAPIHTTPClient(endpoint string, api InternalAPI, certs ...*x509.Certificate) (*http.Client, error) {
	target, err := newInternalAPITarget(endpoint, api)
	if err != nil {
		return nil, err
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	for _, cert := range certs {
		if cert != nil {
			pool.AddCert(cert)
		}
	}
	transport := oktetoHttp.DefaultTransport()
	transport.Proxy = nil
	transport.TLSClientConfig.RootCAs = pool
	transport.TLSClientConfig.ServerName = target.hostname
	transport.TLSClientConfig.InsecureSkipVerify = insecureSkipTLSVerify // skipcq: GSC-G402

	return &http.Client{
		Transport: &internalAPITransport{target: target, base: transport},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// internalAPITarget is where a client sends the requests to the internal API
type internalAPITarget struct {
	// address is the internal IP of the Okteto API with the port of the endpoint
	address string
	// hostname is the hostname the certificate of the internal API is valid for
	hostname string
	// host is the Host header: the hostname, with the port of the endpoint when it has one, as an endpoint with the
	// hostname sends it
	host string
}

func newInternalAPITarget(endpoint string, api InternalAPI) (internalAPITarget, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return internalAPITarget{}, ErrInvalidInternalAPIEndpoint
	}

	hostname := u.Hostname()
	ip := net.ParseIP(hostname)
	if ip != nil {
		hostname = api.Hostname
		if !isValidInternalAPIHostname(hostname) {
			return internalAPITarget{}, ErrInvalidInternalAPIHostname
		}
	} else {
		ip = net.ParseIP(api.IP)
		if ip == nil {
			return internalAPITarget{}, ErrInvalidInternalAPIIP
		}
	}

	port := u.Port()
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else {
		port = "443"
	}
	return internalAPITarget{
		address:  net.JoinHostPort(ip.String(), port),
		hostname: hostname,
		host:     host,
	}, nil
}

// internalAPITransport sends every request to the internal IP of the Okteto API, with the hostname as the Host
type internalAPITransport struct {
	base   http.RoundTripper
	target internalAPITarget
}

func (t *internalAPITransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Host = t.target.address
	req.Host = t.target.host
	return t.base.RoundTrip(req)
}
