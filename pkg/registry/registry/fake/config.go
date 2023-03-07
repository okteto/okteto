package fake

import "crypto/x509"

type FakeConfig struct {
	IsOktetoClusterCfg          bool
	GlobalNamespace             string
	Namespace                   string
	RegistryURL                 string
	UserID                      string
	Token                       string
	InsecureSkipTLSVerifyPolicy bool
	ContextCertificate          *x509.Certificate
}

func (fc FakeConfig) IsOktetoCluster() bool               { return fc.IsOktetoClusterCfg }
func (fc FakeConfig) GetGlobalNamespace() string          { return fc.GlobalNamespace }
func (fc FakeConfig) GetNamespace() string                { return fc.Namespace }
func (fc FakeConfig) GetRegistryURL() string              { return fc.RegistryURL }
func (fc FakeConfig) GetUserID() string                   { return fc.UserID }
func (fc FakeConfig) GetToken() string                    { return fc.Token }
func (fc FakeConfig) IsInsecureSkipTLSVerifyPolicy() bool { return fc.InsecureSkipTLSVerifyPolicy }
func (fc FakeConfig) GetContextCertificate() (*x509.Certificate, error) {
	return fc.ContextCertificate, nil
}
