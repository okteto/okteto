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
)

type Config struct {
	IsOkteto                    bool
	GlobalNamespace             string
	Namespace                   string
	RegistryUrl                 string
	UserId                      string
	Token                       string
	Cert                        string
	InsecureSkipTLSVerifyPolicy bool
	ServerNameOverride          string
	ContextName                 string
	Credential                  struct {
		Username string
		Password string
		Err      error
	}
}

func (c Config) IsOktetoCluster() bool      { return c.IsOkteto }
func (c Config) GetGlobalNamespace() string { return c.GlobalNamespace }
func (c Config) GetNamespace() string       { return c.Namespace }
func (c Config) GetRegistryURL() string     { return c.RegistryUrl }
func (c Config) GetUserID() string          { return c.UserId }
func (c Config) GetToken() string           { return c.Token }
func (c Config) GetContextCertificate() (*x509.Certificate, error) {
	return GetContextCertificateStateless(c.Cert)
}
func (c Config) IsInsecureSkipTLSVerifyPolicy() bool { return c.InsecureSkipTLSVerifyPolicy }
func (Config) GetServerNameOverride() string         { return GetServerNameOverride() }
func (c Config) GetContextName() string              { return c.ContextName }
func (c Config) GetExternalRegistryCredentials(registryHost string) (string, string, error) {
	return c.Credential.Username, c.Credential.Password, c.Credential.Err
	//return GetExternalRegistryCredentials(registryHost)
}
