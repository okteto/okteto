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

package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"
)

// DefaultTransport returns an *http.Transport lifted from http.DefaultTransport
// Main differences vs empty &http.Client{} are http2 preference, min TLS version set to 1.2, timeouts and connection limits.
//
// dev: reason why not doing pointer cloning is because not safe after init():
// - https://github.com/golang/go/issues/26013
// dev: kubernetes project preferred lift vs pointer cloning:
// - https://github.com/kubernetes-retired/go-open-service-broker-client/pull/133
func DefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: 0,
		},
	}
}

type TLSDialFunc func(network string, addr string, config *tls.Config) (TLSConn, error)

func DefaultTLSDial(network string, addr string, config *tls.Config) (TLSConn, error) {
	return tls.Dial(network, addr, config)
}

type TLSConn interface {
	net.Conn
	Handshake() error
}

// StrictSSLTransport returns an *http.Transport with RootCAs set with both the SystemCertPool and the given *x509.Certificates
// If obtaining SystemCertPool fails, it uses an empty *x509.CertPool as base
func StrictSSLTransport(opts *SSLTransportOption) *http.Transport {
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	if opts == nil {
		opts = &SSLTransportOption{}
	}

	for _, cert := range opts.Certs {
		if cert == nil {
			continue
		}
		pool.AddCert(cert)
	}

	toIntercept := Intercept{}
	toIntercept.AppendURLs(opts.URLsToIntercept...)

	transport := DefaultTransport()
	transport.TLSClientConfig.RootCAs = pool

	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if toIntercept.ShouldInterceptAddr(addr) && opts.ServerName != "" {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			transport.TLSClientConfig.ServerName = host
			addr = opts.ServerName
		}

		tlsDial := opts.TLSDial
		if tlsDial == nil {
			tlsDial = DefaultTLSDial
		}
		tlsConn, err := tlsDial("tcp", addr, transport.TLSClientConfig)
		if err != nil {
			return nil, fmt.Errorf("tcp dial failed for %s: %w", addr, err)
		}
		if err := tlsConn.Handshake(); err != nil {
			return nil, fmt.Errorf("tls handshake failed for %s: %w", addr, err)
		}
		return tlsConn, nil
	}

	return transport
}

// InsecureTransport returns an *http.Transport with InsecureSkipVerify set to true in TLSClientConfig
func InsecureTransport() *http.Transport {
	transport := DefaultTransport()
	transport.TLSClientConfig.InsecureSkipVerify = true // skipcq: GSC-G402

	return transport
}
