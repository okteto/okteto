// Copyright 2021 The Okteto Authors
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
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Credentials top body answer
type Credentials struct {
	Credentials Credential
}

// Credential represents an Okteto Space k8s credentials
type Credential struct {
	Server      string `json:"server" yaml:"server"`
	Certificate string `json:"certificate" yaml:"certificate"`
	Token       string `json:"token" yaml:"token"`
	Namespace   string `json:"namespace" yaml:"namespace"`
}

// GetCredentials returns the space config credentials
func GetCredentials(ctx context.Context) (*Credential, error) {
	q := `query{
		credentials(space: ""){
			server, certificate, token, namespace
		},
	}`

	var cred Credentials
	if err := query(ctx, q, &cred); err != nil {
		return nil, err
	}

	if cred.Credentials.Server == "" {
		return nil, fmt.Errorf("%s is not available. Please, retry again in a few minutes", GetURL())
	}

	return &cred.Credentials, nil
}

// GetClusterContext returns the k8s context names given an okteto URL
func GetClusterContext() string {
	u, _ := url.Parse(GetURL())
	return strings.ReplaceAll(u.Host, ".", "_")
}
