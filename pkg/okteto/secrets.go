// Copyright 2022 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

type userClient struct {
	client *graphql.Client
}

func newUserClient(client *graphql.Client) *userClient {
	return &userClient{client: client}
}

//GetSecrets returns the secrets from Okteto API
func (c *OktetoClient) GetSecrets(ctx context.Context) ([]types.Secret, error) {
	var queryStruct struct {
		Secrets []struct {
			Name  graphql.String
			Value graphql.String
		} `graphql:"getGitDeploySecrets"`
	}

	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		return nil, err
	}

	secrets := make([]types.Secret, 0)
	for _, secret := range queryStruct.Secrets {
		if !strings.Contains(string(secret.Name), ".") {
			secrets = append(secrets, types.Secret{
				Name:  string(secret.Name),
				Value: string(secret.Value),
			})
		}
	}
	return secrets, nil
}

//GetSecrets returns the secrets from Okteto API
func (c *userClient) GetContext(ctx context.Context) (*types.UserContext, error) {
	var queryStruct struct {
		User struct {
			Id              graphql.String
			Name            graphql.String
			Namespace       graphql.String
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
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"globalNamespace\" on type \"me\"") {
			return c.deprecatedGetUserContext(ctx)
		}
		if strings.Contains(err.Error(), "Cannot query field \"telemetryEnabled\" on type \"me\"") {
			return c.deprecatedGetUserContext(ctx)
		}
		return nil, err
	}

	secrets := make([]types.Secret, 0)
	for _, secret := range queryStruct.Secrets {
		if !strings.Contains(string(secret.Name), ".") {
			secrets = append(secrets, types.Secret{
				Name:  string(secret.Name),
				Value: string(secret.Value),
			})
		}
	}

	globalNamespace := getGlobalNamespace(string(queryStruct.User.GlobalNamespace))
	analytics := bool(queryStruct.User.Analytics)

	result := &types.UserContext{
		User: types.User{
			ID:              string(queryStruct.User.Id),
			Name:            string(queryStruct.User.Name),
			Namespace:       string(queryStruct.User.Namespace),
			Email:           string(queryStruct.User.Email),
			ExternalID:      string(queryStruct.User.ExternalID),
			Token:           string(queryStruct.User.Token),
			New:             bool(queryStruct.User.New),
			Registry:        string(queryStruct.User.Registry),
			Buildkit:        string(queryStruct.User.Buildkit),
			Certificate:     string(queryStruct.User.Certificate),
			GlobalNamespace: globalNamespace,
			Analytics:       analytics,
		},
		Secrets: secrets,
		Credentials: types.Credential{
			Server:      string(queryStruct.Cred.Server),
			Certificate: string(queryStruct.Cred.Certificate),
			Token:       string(queryStruct.Cred.Token),
			Namespace:   string(queryStruct.Cred.Namespace),
		},
	}
	return result, nil
}

func (c *userClient) deprecatedGetUserContext(ctx context.Context) (*types.UserContext, error) {
	var queryStruct struct {
		User struct {
			Id          graphql.String
			Name        graphql.String
			Namespace   graphql.String
			Email       graphql.String
			ExternalID  graphql.String `graphql:"externalID"`
			Token       graphql.String
			New         graphql.Boolean
			Registry    graphql.String
			Buildkit    graphql.String
			Certificate graphql.String
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
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	secrets := make([]types.Secret, 0)
	for _, secret := range queryStruct.Secrets {
		if !strings.Contains(string(secret.Name), ".") {
			secrets = append(secrets, types.Secret{
				Name:  string(secret.Name),
				Value: string(secret.Value),
			})
		}
	}
	result := &types.UserContext{
		User: types.User{
			ID:              string(queryStruct.User.Id),
			Name:            string(queryStruct.User.Name),
			Namespace:       string(queryStruct.User.Namespace),
			Email:           string(queryStruct.User.Email),
			ExternalID:      string(queryStruct.User.ExternalID),
			Token:           string(queryStruct.User.Token),
			New:             bool(queryStruct.User.New),
			Registry:        string(queryStruct.User.Registry),
			Buildkit:        string(queryStruct.User.Buildkit),
			Certificate:     string(queryStruct.User.Certificate),
			GlobalNamespace: DefaultGlobalNamespace,
			Analytics:       true,
		},
		Secrets: secrets,
		Credentials: types.Credential{
			Server:      string(queryStruct.Cred.Server),
			Certificate: string(queryStruct.Cred.Certificate),
			Token:       string(queryStruct.Cred.Token),
			Namespace:   string(queryStruct.Cred.Namespace),
		},
	}
	return result, nil
}
