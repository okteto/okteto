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
	"strings"

	"github.com/shurcooL/graphql"
)

//Secret represents a secret
type Secret struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type SecretsAndCredentialToken struct {
	Secrets     []Secret   `json:"secrets,omitempty"`
	Credentials Credential `json:"credentials,omitempty"`
}

//GetSecrets returns the secrets from Okteto API
func (c *OktetoClient) GetSecrets(ctx context.Context) ([]Secret, error) {
	var query struct {
		Secrets []struct {
			Name  graphql.String
			Value graphql.String
		} `graphql:"getGitDeploySecrets"`
	}

	err := c.client.Query(ctx, &query, nil)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	secrets := make([]Secret, 0)
	for _, secret := range query.Secrets {
		if !strings.Contains(string(secret.Name), ".") {
			secrets = append(secrets, Secret{
				Name:  string(secret.Name),
				Value: string(secret.Value),
			})
		}
	}
	return secrets, nil
}

//GetSecrets returns the secrets from Okteto API
func (c *OktetoClient) GetSecretsAndKubeCredentials(ctx context.Context) (*SecretsAndCredentialToken, error) {
	var query struct {
		Secrets []struct {
			Name  graphql.String
			Value graphql.String
		} `graphql:"getGitDeploySecrets"`
		Space struct {
			Server      graphql.String
			Certificate graphql.String
			Token       graphql.String
			Namespace   graphql.String
		} `graphql:"credentials(space: $cred)"`
	}
	variables := map[string]interface{}{
		"cred": graphql.String(""),
	}
	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, translateAPIErr(err)
	}

	secrets := make([]Secret, 0)
	for _, secret := range query.Secrets {
		if !strings.Contains(string(secret.Name), ".") {
			secrets = append(secrets, Secret{
				Name:  string(secret.Name),
				Value: string(secret.Value),
			})
		}
	}
	result := &SecretsAndCredentialToken{
		Secrets: secrets,
		Credentials: Credential{
			Server:      string(query.Space.Server),
			Certificate: string(query.Space.Certificate),
			Token:       string(query.Space.Token),
			Namespace:   string(query.Space.Namespace),
		},
	}
	return result, nil
}
