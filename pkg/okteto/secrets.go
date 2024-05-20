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
	"encoding/base64"
	"fmt"
	"strings"

	dockertypes "github.com/docker/cli/cli/config/types"
	dockercredentials "github.com/docker/docker-credential-helpers/credentials"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	"gopkg.in/yaml.v3"
)

type userClient struct {
	client graphqlClientInterface
}

func newUserClient(client graphqlClientInterface) *userClient {
	return &userClient{client: client}
}

type getContextQuery struct {
	Cred              credQuery        `graphql:"credentials(space: $cred)"`
	User              userQuery        `graphql:"user"`
	PlatformVariables []variablesQuery `graphql:"getGitDeploySecrets"`
}

type getVariablesQuery struct {
	Variables []variablesQuery `graphql:"getGitDeploySecrets"`
}

type getContextFileQuery struct {
	ContextFileJSON string `graphql:"contextFile"`
}

type getRegistryCredentialsQuery struct {
	RegistryCredentials registryCredsQuery `graphql:"registryCredentials(registryUrl: $regHost)"`
}

type userQuery struct {
	Id              graphql.String
	Name            graphql.String
	Namespace       graphql.String
	Email           graphql.String
	ExternalID      graphql.String `graphql:"externalID"`
	Token           graphql.String
	Registry        graphql.String
	Buildkit        graphql.String
	Certificate     graphql.String
	GlobalNamespace graphql.String `graphql:"globalNamespace"`
	New             graphql.Boolean
	Analytics       graphql.Boolean `graphql:"telemetryEnabled"`
}

type credQuery struct {
	Server      graphql.String
	Certificate graphql.String
	Token       graphql.String
	Namespace   graphql.String
}

type registryCredsQuery struct {
	Username      graphql.String
	Password      graphql.String
	Auth          graphql.String
	Serveraddress graphql.String
	Identitytoken graphql.String
	Registrytoken graphql.String
}

type metadataQuery struct {
	Metadata []metadataQueryItem `graphql:"metadata(namespace: $namespace)"`
}

type variablesQuery struct {
	Name  graphql.String
	Value graphql.String
}

type metadataQueryItem struct {
	Name  graphql.String
	Value graphql.String
}

type contextFileJSON struct {
	Contexts map[string]struct {
		Certificate string `yaml:"certificate"`
	} `yaml:"contexts"`
}

// GetContext returns the user context from Okteto API
func (c *userClient) GetContext(ctx context.Context, ns string) (*types.UserContext, error) {
	var queryStruct getContextQuery
	variables := map[string]interface{}{
		"cred": graphql.String(ns),
	}
	err := query(ctx, &queryStruct, variables, c.client)
	if err != nil {
		return nil, err
	}

	platformVars := make([]env.Var, 0)
	for _, v := range queryStruct.PlatformVariables {
		if !strings.Contains(string(v.Name), ".") {
			platformVars = append(platformVars, env.Var{
				Name:  string(v.Name),
				Value: string(v.Value),
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
		PlatformVariables: platformVars,
		Credentials: types.Credential{
			Server:      string(queryStruct.Cred.Server),
			Certificate: string(queryStruct.Cred.Certificate),
			Token:       string(queryStruct.Cred.Token),
			Namespace:   string(queryStruct.Cred.Namespace),
		},
	}
	return result, nil
}

// GetOktetoPlatformVariables returns the user and cluster variables from Okteto API
func (c *userClient) GetOktetoPlatformVariables(ctx context.Context) ([]env.Var, error) {
	var queryStruct getVariablesQuery
	err := query(ctx, &queryStruct, nil, c.client)
	if err != nil {
		return nil, err
	}

	vars := make([]env.Var, 0)
	for _, v := range queryStruct.Variables {
		if !strings.Contains(string(v.Name), ".") {
			vars = append(vars, env.Var{
				Name:  string(v.Name),
				Value: string(v.Value),
			})
		}
	}

	return vars, nil
}

func (c *userClient) GetClusterCertificate(ctx context.Context, cluster, ns string) ([]byte, error) {
	var queryStruct getContextFileQuery
	if err := query(ctx, &queryStruct, nil, c.client); err != nil {
		return nil, err
	}

	payload, err := base64.StdEncoding.DecodeString(queryStruct.ContextFileJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode credentials file: %w", err)
	}

	var file contextFileJSON
	if err := yaml.Unmarshal(payload, &file); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials file: %w", err)
	}

	conf, ok := file.Contexts[cluster]
	if !ok {
		return nil, fmt.Errorf("context-not-found")
	}
	if conf.Certificate == "" {
		return nil, fmt.Errorf("context has no certificate")
	}

	b, err := base64.StdEncoding.DecodeString(conf.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode context certificate: %w", err)
	}

	return b, nil
}

// GetClusterMetadata returns the metadata with the cluster configuration
func (c *userClient) GetClusterMetadata(ctx context.Context, ns string) (types.ClusterMetadata, error) {
	var queryStruct metadataQuery
	vars := map[string]interface{}{
		"namespace": graphql.String(ns),
	}

	err := query(ctx, &queryStruct, vars, c.client)

	if err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"metadata\" on type \"Query\"") {
			// when query is not present by backend return the default cluster metadata
			return types.ClusterMetadata{
				PipelineRunnerImage: constants.OktetoPipelineRunnerImage,
			}, nil
		}
		return types.ClusterMetadata{}, err
	}

	metadata := types.ClusterMetadata{}

	// metadata := make(types.ClusterMetadata, len(queryStruct.Metadata))
	for _, v := range queryStruct.Metadata {
		if v.Value == "" {
			continue
		}
		switch v.Name {
		case "internalCertificateBase64":
			cert, err := base64.StdEncoding.DecodeString(string(v.Value))
			if err != nil {
				return metadata, err
			}
			metadata.Certificate = cert
		case "internalIngressControllerNetworkAddress":
			metadata.ServerName = string(v.Value)
		case "pipelineRunnerImage":
			metadata.PipelineRunnerImage = string(v.Value)
		case "isTrial":
			metadata.IsTrialLicense = string(v.Value) == "true"
		case "companyName":
			metadata.CompanyName = string(v.Value)
		case "buildkitInternalIP":
			metadata.BuildKitInternalIP = string(v.Value)
		case "publicDomain":
			metadata.PublicDomain = string(v.Value)
		}
	}
	if metadata.PipelineRunnerImage == "" {
		return metadata, fmt.Errorf("missing metadata")
	}
	return metadata, nil
}

func (c *userClient) GetRegistryCredentials(ctx context.Context, host string) (dockertypes.AuthConfig, error) {
	var queryStruct getRegistryCredentialsQuery
	vars := map[string]interface{}{
		"regHost": graphql.String(host),
	}
	err := query(ctx, &queryStruct, vars, c.client)

	if err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"registryCredentials\" on type \"Query\"") {
			return dockertypes.AuthConfig{}, dockercredentials.NewErrCredentialsNotFound()
		}
		if errors.IsNotFound(err) {
			return dockertypes.AuthConfig{}, dockercredentials.NewErrCredentialsNotFound()
		}
		return dockertypes.AuthConfig{}, err
	}
	return dockertypes.AuthConfig{
		Username:      string(queryStruct.RegistryCredentials.Username),
		Password:      string(queryStruct.RegistryCredentials.Password),
		Auth:          string(queryStruct.RegistryCredentials.Auth),
		ServerAddress: string(queryStruct.RegistryCredentials.Serveraddress),
		RegistryToken: string(queryStruct.RegistryCredentials.Registrytoken),
		IdentityToken: string(queryStruct.RegistryCredentials.Identitytoken),
	}, nil

}
