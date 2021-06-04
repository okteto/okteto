// Copyright 2020 The Okteto Authors
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
	"strings"
)

//Secrets represents a list of secrets
type Secrets struct {
	Secrets []Secret `json:"getGitDeploySecrets,omitempty"`
}

//Secret represents a secret
type Secret struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

var (
	secrets          []Secret
	retrievedSecrets bool
)

//GetSecrets returns the secrets from Okteto API
func GetSecrets(ctx context.Context) ([]Secret, error) {
	if !retrievedSecrets {
		q := `query{
			getGitDeploySecrets{
				name,value
			},
		}`

		var body Secrets
		if err := query(ctx, q, &body); err != nil {
			return nil, err
		}
		secrets = make([]Secret, 0)
		for _, secret := range body.Secrets {
			if !strings.Contains(secret.Name, ".") {
				secrets = append(secrets, secret)
			}
		}
		retrievedSecrets = true
		return secrets, nil
	}
	return secrets, nil
}
