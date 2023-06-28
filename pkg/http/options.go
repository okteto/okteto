package http

import "crypto/x509"

type SSLTransportOption struct {
	Certs           []*x509.Certificate
	ServerName      string
	URLsToIntercept []string
	TLSDial         TLSDialFunc
}
