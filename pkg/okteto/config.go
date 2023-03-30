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

import "crypto/x509"

type Config struct{}

func (Config) IsOktetoCluster() bool                             { return IsOkteto() }
func (Config) GetGlobalNamespace() string                        { return Context().GlobalNamespace }
func (Config) GetNamespace() string                              { return Context().Namespace }
func (Config) GetRegistryURL() string                            { return Context().Registry }
func (Config) GetUserID() string                                 { return Context().UserID }
func (Config) GetToken() string                                  { return Context().Token }
func (Config) GetContextCertificate() (*x509.Certificate, error) { return GetContextCertificate() }
func (Config) IsInsecureSkipTLSVerifyPolicy() bool               { return Context().IsInsecure }
