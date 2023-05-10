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
	"context"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

type userClient struct {
	client graphqlClientInterface
}

func newUserClient(client graphqlClientInterface) *userClient {
	return &userClient{client: client}
}

type getContextQuery struct {
	User    userQuery     `graphql:"user"`
	Secrets []secretQuery `graphql:"getGitDeploySecrets"`
	Cred    credQuery     `graphql:"credentials(space: $cred)"`
}

type getSecretsQuery struct {
	Secrets []secretQuery `graphql:"getGitDeploySecrets"`
}

type getDeprecatedContextQuery struct {
	User    deprecatedUserQuery `graphql:"user"`
	Secrets []secretQuery       `graphql:"getGitDeploySecrets"`
	Cred    credQuery           `graphql:"credentials(space: $cred)"`
}

type userQuery struct {
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
}

// TODO: Remove this code when users are in okteto chart > 0.10.8
type deprecatedUserQuery struct {
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
}

type secretQuery struct {
	Name  graphql.String
	Value graphql.String
}

type credQuery struct {
	Server      graphql.String
	Certificate graphql.String
	Token       graphql.String
	Namespace   graphql.String
}

// GetSecrets returns the secrets from Okteto API
func (c *userClient) GetContext(ctx context.Context, ns string) (*types.UserContext, error) {
	var queryStruct getContextQuery
	variables := map[string]interface{}{
		"cred": graphql.String(ns),
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

// GetSecrets returns the secrets from Okteto API
func (c *userClient) GetUserSecrets(ctx context.Context) ([]types.Secret, error) {
	var queryStruct getSecretsQuery
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

// TODO: Remove this code when users are in okteto chart > 0.10.8
func (c *userClient) deprecatedGetUserContext(ctx context.Context) (*types.UserContext, error) {
	var queryStruct getDeprecatedContextQuery
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
			GlobalNamespace: constants.DefaultGlobalNamespace,
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
