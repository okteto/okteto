package http

import (
	"crypto/x509"
	"net/http"
)

// StrictSSLHTTPClient receives multiple *x509.Certificate and returns an *http.Client with a StrictSSLTransport
func StrictSSLHTTPClient(certs ...*x509.Certificate) *http.Client {
	transport := StrictSSLTransport(certs...)

	return &http.Client{
		Transport: transport,
	}
}

// InsecureHTTPClient returns an *http.Client with an InsecureTransport
func InsecureHTTPClient() *http.Client {
	transport := InsecureTransport()

	return &http.Client{
		Transport: transport,
	}
}
