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

type UserContext struct {
	User        User       `json:"user,omitempty"`
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
func (c *OktetoClient) GetUserContext(ctx context.Context) (*UserContext, error) {
	var query struct {
		User struct {
			Id              graphql.String
			Name            graphql.String
			Email           graphql.String
			ExternalID      graphql.String `graphql:"externalID"`
			Token           graphql.String
			New             graphql.Boolean
			Registry        graphql.String
			Buildkit        graphql.String
			Certificate     graphql.String
			GlobalNamespace graphql.String  `graphql:"globalNamespace"`
			Analytics       graphql.Boolean `graphql:"telemetryEnabled"`
		} `graphql:"user"`
		Secrets []struct {
			Name  graphql.String
			Value graphql.String
		} `graphql:"getGitDeploySecrets"`
		Cred struct {
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

	globalNamespace := getGlobalNamespace(string(query.User.GlobalNamespace))
	analytics := bool(query.User.Analytics)
	if IsOktetoCloud() {
		analytics = true
	}
	result := &UserContext{
		User: User{
			ID:              string(query.User.Id),
			Name:            string(query.User.Name),
			Email:           string(query.User.Email),
			ExternalID:      string(query.User.ExternalID),
			Token:           string(query.User.Token),
			New:             bool(query.User.New),
			Registry:        string(query.User.Registry),
			Buildkit:        string(query.User.Buildkit),
			Certificate:     string(query.User.Certificate),
			GlobalNamespace: globalNamespace,
			Analytics:       analytics,
		},
		Secrets: secrets,
		Credentials: Credential{
			Server:      string(query.Cred.Server),
			Certificate: string(query.Cred.Certificate),
			Token:       string(query.Cred.Token),
			Namespace:   string(query.Cred.Namespace),
		},
	}
	return result, nil
}
