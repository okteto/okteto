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
// Main differentes vs empty &http.Client{} are http2 preference, min TLS version set to 1.2, timeouts and connection limits.
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

		conn, err := tls.Dial("tcp", addr, transport.TLSClientConfig)
		if err != nil {
			return nil, fmt.Errorf("tcp dial failed for %s: %w", addr, err)
		}
		if err := conn.Handshake(); err != nil {
			return nil, fmt.Errorf("tls handshake failed for %s: %w", addr, err)
		}
		return conn, err
	}

	return transport
}

// InsecureTransport returns an *http.Transport with InsecureSkipVerify set to true in TLSClientConfig
func InsecureTransport() *http.Transport {
	transport := DefaultTransport()
	transport.TLSClientConfig.InsecureSkipVerify = true // skipcq: GSC-G402

	return transport
}
