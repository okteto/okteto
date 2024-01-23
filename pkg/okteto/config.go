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

package okteto

import (
	"crypto/x509"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type ConfigStateless struct {
	GetTokenFn                  func(string) (string, error)
	GlobalNamespace             string
	Namespace                   string
	RegistryUrl                 string
	UserId                      string
	Token                       string
	Cert                        string
	ServerNameOverride          string
	ContextName                 string
	InsecureSkipTLSVerifyPolicy bool
	IsOkteto                    bool
}

func (c ConfigStateless) IsOktetoCluster() bool      { return c.IsOkteto }
func (c ConfigStateless) GetGlobalNamespace() string { return c.GlobalNamespace }
func (c ConfigStateless) GetNamespace() string       { return c.Namespace }
func (c ConfigStateless) GetRegistryURL() string     { return c.RegistryUrl }
func (c ConfigStateless) GetUserID() string          { return c.UserId }
func (c ConfigStateless) GetToken() string           { return c.Token }
func (c ConfigStateless) GetContextCertificate() (*x509.Certificate, error) {
	return GetContextCertificateStateless(c.Cert)
}
func (c ConfigStateless) IsInsecureSkipTLSVerifyPolicy() bool { return c.InsecureSkipTLSVerifyPolicy }
func (ConfigStateless) GetServerNameOverride() string         { return GetServerNameOverride() }
func (c ConfigStateless) GetContextName() string              { return c.ContextName }
func (c ConfigStateless) GetExternalRegistryCredentials(registryHost string) (string, string, error) {
	ocfg := &ClientCfg{
		CtxName: c.ContextName,
		Token:   c.Token,
		Cert:    c.Cert,
	}
	client, err := NewOktetoClientStateless(ocfg)
	if err != nil {
		oktetoLog.Debugf("failed to create okteto client for getting registry credentials: %s", err.Error())
		return "", "", err
	}
	return GetExternalRegistryCredentialsStateless(registryHost, c.IsOkteto, client)
}

type Config struct{}

func (Config) IsOktetoCluster() bool                             { return IsOkteto() }
func (Config) GetGlobalNamespace() string                        { return GetContext().GlobalNamespace }
func (Config) GetNamespace() string                              { return GetContext().Namespace }
func (Config) GetRegistryURL() string                            { return GetContext().Registry }
func (Config) GetUserID() string                                 { return GetContext().UserID }
func (Config) GetToken() string                                  { return GetContext().Token }
func (Config) GetContextCertificate() (*x509.Certificate, error) { return GetContextCertificate() }
func (Config) IsInsecureSkipTLSVerifyPolicy() bool               { return GetContext().IsInsecure }
func (Config) GetServerNameOverride() string                     { return GetServerNameOverride() }
func (Config) GetContextName() string                            { return GetContext().Name }
func (Config) GetExternalRegistryCredentials(registryHost string) (string, string, error) {
	return GetExternalRegistryCredentials(registryHost)
}
