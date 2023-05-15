package http

import (
	"net/http"
)

// StrictSSLHTTPClient receives multiple *x509.Certificate and returns an *http.Client with a StrictSSLTransport
func StrictSSLHTTPClient(opts *SSLTransportOption) *http.Client {
	transport := StrictSSLTransport(opts)

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
