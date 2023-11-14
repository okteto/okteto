package http

import "crypto/x509"

type SSLTransportOption struct {
	TLSDial         TLSDialFunc
	ServerName      string
	Certs           []*x509.Certificate
	URLsToIntercept []string
}
