package http

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"
)

// DefaultTransport returns an *http.Transport lifted from http.DefaultTransport
// Main differentes vs empty &http.Client{} are http2 preference, min TLS version set to 1.2, timeouts and connection limits.
//
// dev: reason why not doing pointer cloning is because not safe after init():
// - https://github.com/golang/go/issues/26013
// dev: kubernetes project preffered lift vs pointer cloning:
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

// StrictSSLHTTPClient returns an *http.Transport with RootCAs set with both the SystemCertPool and the given *x509.Certificates
// If obtaining SystemCertPool fails, it uses an empty *x509.CertPool as base
func StrictSSLTransport(certs ...*x509.Certificate) *http.Transport {
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	for _, cert := range certs {
		pool.AddCert(cert)
	}

	transport := DefaultTransport()
	transport.TLSClientConfig.RootCAs = pool

	return transport
}

// InsecureTransport returns an *http.Transport with InsecureSkipVerify set to true in TLSClientConfig
func InsecureTransport() *http.Transport {
	transport := DefaultTransport()
	transport.TLSClientConfig.InsecureSkipVerify = true // skipcq: GSC-G402

	return transport
}
