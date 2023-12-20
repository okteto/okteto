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
	"errors"
	"fmt"
	"testing"

	dockertypes "github.com/docker/cli/cli/config/types"
	dockercredentials "github.com/docker/docker-credential-helpers/credentials"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	"github.com/stretchr/testify/assert"
)

func TestGetContext(t *testing.T) {
	globalNsErr := fmt.Errorf("Cannot query field \"globalNamespace\" on type \"me\"")
	telemetryEnabledErr := fmt.Errorf("Cannot query field \"telemetryEnabled\" on type \"me\"")
	tokenExpiredErr := fmt.Errorf("non-200 OK status code: 401 Unauthorized body: \"not-authorized: token is expired\\n\"")
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		userContext *types.UserContext
		err         error
	}
	testCases := []struct {
		expected expected
		cfg      input
		name     string
	}{
		{
			name: "error in graphql",
			cfg: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				userContext: nil,
				err:         assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getContextQuery{
						User: userQuery{
							Id:              "id",
							Name:            "name",
							Namespace:       "ns",
							Email:           "email",
							ExternalID:      "externalID",
							Token:           "token",
							New:             false,
							Registry:        "registry.com",
							Buildkit:        "buildkit.com",
							Certificate:     "cert",
							GlobalNamespace: "globalNs",
							Analytics:       false,
						},
						Secrets: []secretQuery{
							{
								Name:  "name",
								Value: "value",
							},
						},
						Cred: credQuery{
							Server:      "my-server.com",
							Certificate: "cert",
							Token:       "token",
							Namespace:   "ns",
						},
					},
				},
			},
			expected: expected{
				userContext: &types.UserContext{
					User: types.User{
						ID:              "id",
						Name:            "name",
						Namespace:       "ns",
						Email:           "email",
						ExternalID:      "externalID",
						Token:           "token",
						New:             false,
						Registry:        "registry.com",
						Buildkit:        "buildkit.com",
						Certificate:     "cert",
						GlobalNamespace: "globalNs",
						Analytics:       false,
					},
					Secrets: []types.Secret{
						{
							Name:  "name",
							Value: "value",
						},
					},
					Credentials: types.Credential{
						Server:      "my-server.com",
						Certificate: "cert",
						Token:       "token",
						Namespace:   "ns",
					},
				},
				err: nil,
			},
		},
		{
			name: "graphql response is an action empty globalNS",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getContextQuery{
						User: userQuery{
							Id:              "id",
							Name:            "name",
							Namespace:       "ns",
							Email:           "email",
							ExternalID:      "externalID",
							Token:           "token",
							New:             false,
							Registry:        "registry.com",
							Buildkit:        "buildkit.com",
							Certificate:     "cert",
							GlobalNamespace: "",
							Analytics:       false,
						},
						Secrets: []secretQuery{
							{
								Name:  "name",
								Value: "value",
							},
						},
						Cred: credQuery{
							Server:      "my-server.com",
							Certificate: "cert",
							Token:       "token",
							Namespace:   "ns",
						},
					},
				},
			},
			expected: expected{
				userContext: &types.UserContext{
					User: types.User{
						ID:              "id",
						Name:            "name",
						Namespace:       "ns",
						Email:           "email",
						ExternalID:      "externalID",
						Token:           "token",
						New:             false,
						Registry:        "registry.com",
						Buildkit:        "buildkit.com",
						Certificate:     "cert",
						GlobalNamespace: constants.DefaultGlobalNamespace,
						Analytics:       false,
					},
					Secrets: []types.Secret{
						{
							Name:  "name",
							Value: "value",
						},
					},
					Credentials: types.Credential{
						Server:      "my-server.com",
						Certificate: "cert",
						Token:       "token",
						Namespace:   "ns",
					},
				},
				err: nil,
			},
		},
		{
			name: "globalNamespace not in response",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: nil,
					err:         globalNsErr,
				},
			},
			expected: expected{
				userContext: nil,
				err:         globalNsErr,
			},
		},
		{
			name: "telemetryEnabled not in response",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: nil,
					err:         telemetryEnabledErr,
				},
			},
			expected: expected{
				userContext: nil,
				err:         telemetryEnabledErr,
			},
		},
		{
			name: "error because token is expired",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: nil,
					err:         tokenExpiredErr,
				},
			},
			expected: expected{
				userContext: nil,
				err:         oktetoErrors.ErrTokenExpired,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uc := &userClient{
				client: tc.cfg.client,
			}
			userContext, err := uc.GetContext(context.Background(), "")
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.userContext, userContext)
		})
	}
}

func TestGetUserSecrets(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		err         error
		userSecrets []types.Secret
	}
	testCases := []struct {
		cfg      input
		name     string
		expected expected
	}{
		{
			name: "error in graphql",
			cfg: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				userSecrets: nil,
				err:         assert.AnError,
			},
		},
		{
			name: "query get user secrets",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getSecretsQuery{
						Secrets: []secretQuery{
							{
								Name:  "password",
								Value: "test",
							},
							{
								Name:  "pass.word",
								Value: "test",
							},
						},
					},
				},
			},
			expected: expected{
				userSecrets: []types.Secret{
					{
						Name:  "password",
						Value: "test",
					},
				},
			},
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uc := &userClient{
				client: tc.cfg.client,
			}
			userSecrets, err := uc.GetUserSecrets(ctx)
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.userSecrets, userSecrets)
		})
	}
}

func TestGetDeprecatedContext(t *testing.T) {
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		userContext *types.UserContext
		err         error
	}
	testCases := []struct {
		expected expected
		cfg      input
		name     string
	}{
		{
			name: "error in graphql",
			cfg: input{
				client: &fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				userContext: nil,
				err:         assert.AnError,
			},
		},
		{
			name: "graphql response is an action",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getDeprecatedContextQuery{
						User: deprecatedUserQuery{
							Id:          "id",
							Name:        "name",
							Namespace:   "ns",
							Email:       "email",
							ExternalID:  "externalID",
							Token:       "token",
							New:         false,
							Registry:    "registry.com",
							Buildkit:    "buildkit.com",
							Certificate: "cert",
						},
						Secrets: []secretQuery{
							{
								Name:  "name",
								Value: "value",
							},
						},
						Cred: credQuery{
							Server:      "my-server.com",
							Certificate: "cert",
							Token:       "token",
							Namespace:   "ns",
						},
					},
				},
			},
			expected: expected{
				userContext: &types.UserContext{
					User: types.User{
						ID:              "id",
						Name:            "name",
						Namespace:       "ns",
						Email:           "email",
						ExternalID:      "externalID",
						Token:           "token",
						New:             false,
						Registry:        "registry.com",
						Buildkit:        "buildkit.com",
						Certificate:     "cert",
						GlobalNamespace: constants.DefaultGlobalNamespace,
						Analytics:       true,
					},
					Secrets: []types.Secret{
						{
							Name:  "name",
							Value: "value",
						},
					},
					Credentials: types.Credential{
						Server:      "my-server.com",
						Certificate: "cert",
						Token:       "token",
						Namespace:   "ns",
					},
				},
				err: nil,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uc := &userClient{
				client: tc.cfg.client,
			}
			userContext, err := uc.deprecatedGetUserContext(context.Background())
			assert.ErrorIs(t, err, tc.expected.err)
			assert.Equal(t, tc.expected.userContext, userContext)
		})
	}
}

func TestGetClusterMetadata(t *testing.T) {
	ctx := context.Background()
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		metadata  types.ClusterMetadata
		expectErr bool
	}
	testCases := []struct {
		name     string
		cfg      input
		expected expected
	}{
		{
			name: "skips error if schema does not match and return default values",
			cfg: input{
				client: &fakeGraphQLClient{
					err: fmt.Errorf("Cannot query field \"metadata\" on type \"Query\""),
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					PipelineRunnerImage: "okteto/pipeline-runner:1.0.2",
				},
			},
		},
		{
			name: "returns other errors",
			cfg: input{
				client: &fakeGraphQLClient{
					err: fmt.Errorf("this is my error. There are many like it but this one is mine"),
				},
			},
			expected: expected{
				expectErr: true,
			},
		},
		{
			name: "all properties are returned",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &metadataQuery{
						Metadata: []metadataQueryItem{
							{
								Name:  "internalCertificateBase64",
								Value: graphql.String(base64.StdEncoding.EncodeToString([]byte("cert"))),
							},
							{
								Name:  "internalIngressControllerNetworkAddress",
								Value: "1.1.1.1",
							},
							{
								Name:  "pipelineInstallerImage",
								Value: "installer-image",
							},
							{
								Name:  "pipelineRunnerImage",
								Value: "installer-runner-image",
							},
							{
								Name:  "buildkitInternalIP",
								Value: "10.10.10.10",
							},
							{
								Name:  "publicDomain",
								Value: "test.okteto.com",
							},
						},
					},
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					Certificate:         []byte("cert"),
					ServerName:          "1.1.1.1",
					PipelineRunnerImage: "installer-runner-image",
					BuildKitInternalIP:  "10.10.10.10",
					PublicDomain:        "test.okteto.com",
				},
			},
		},
		{
			name: "pipelineRunnerImage can't be empty",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &metadataQuery{
						Metadata: []metadataQueryItem{
							{
								Name:  "internalCertificateBase64",
								Value: graphql.String(base64.StdEncoding.EncodeToString([]byte("cert"))),
							},
							{
								Name:  "internalIngressControllerNetworkAddress",
								Value: "1.1.1.1",
							},
						},
					},
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					Certificate: []byte("cert"),
					ServerName:  "1.1.1.1",
				},
				expectErr: true,
			},
		},
		{
			name: "empty companyName",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &metadataQuery{
						Metadata: []metadataQueryItem{
							{
								Name:  "pipelineRunnerImage",
								Value: "installer-runner-image",
							},
							{
								Name:  "companyName",
								Value: "",
							},
						},
					},
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					CompanyName:         "",
					PipelineRunnerImage: "installer-runner-image",
				},
			},
		},
		{
			name: "has companyName",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &metadataQuery{
						Metadata: []metadataQueryItem{
							{
								Name:  "pipelineRunnerImage",
								Value: "installer-runner-image",
							},
							{
								Name:  "companyName",
								Value: "test",
							},
						},
					},
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					CompanyName:         "test",
					PipelineRunnerImage: "installer-runner-image",
				},
			},
		},
		{
			name: "false isTrial",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &metadataQuery{
						Metadata: []metadataQueryItem{
							{
								Name:  "pipelineRunnerImage",
								Value: "installer-runner-image",
							},
							{
								Name:  "isTrial",
								Value: "false",
							},
						},
					},
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					IsTrialLicense:      false,
					PipelineRunnerImage: "installer-runner-image",
				},
			},
		},
		{
			name: "true isTrial",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &metadataQuery{
						Metadata: []metadataQueryItem{
							{
								Name:  "pipelineRunnerImage",
								Value: "installer-runner-image",
							},
							{
								Name:  "isTrial",
								Value: "true",
							},
						},
					},
				},
			},
			expected: expected{
				metadata: types.ClusterMetadata{
					IsTrialLicense:      true,
					PipelineRunnerImage: "installer-runner-image",
				},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			uc := &userClient{
				client: tc.cfg.client,
			}
			result, err := uc.GetClusterMetadata(ctx, "")
			if tc.expected.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expected.metadata.Certificate, result.Certificate)
			assert.Equal(t, tc.expected.metadata.ServerName, result.ServerName)
			assert.Equal(t, tc.expected.metadata.PipelineRunnerImage, result.PipelineRunnerImage)
			assert.Equal(t, tc.expected.metadata.CompanyName, result.CompanyName)
			assert.Equal(t, tc.expected.metadata.IsTrialLicense, result.IsTrialLicense)

		})
	}

}

func TestGetClusterCertificate(t *testing.T) {
	ctx := context.Background()

	cluster := "https://okteto.mycluster.dev.okteto.net"
	testCert := []byte("this-is-my-cert")

	contextFile := func(cluster string, cert []byte) string {
		testCertBase64 := base64.StdEncoding.EncodeToString(cert)
		f := fmt.Sprintf(`{"contexts": {"%s": {"certificate": "%s"}}}`, cluster, testCertBase64)
		return base64.StdEncoding.EncodeToString([]byte(f))
	}
	type input struct {
		client *fakeGraphQLClient
	}
	type expected struct {
		cert      []byte
		expectErr bool
	}
	testCases := []struct {
		name     string
		cfg      input
		expected expected
	}{
		{
			name: "happy path",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getContextFileQuery{
						ContextFileJSON: contextFile(cluster, testCert),
					},
				},
			},
			expected: expected{
				cert: testCert,
			},
		},
		{
			name: "cluster not found",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getContextFileQuery{
						ContextFileJSON: contextFile("incorrect-cluster", testCert),
					},
				},
			},
			expected: expected{
				expectErr: true,
			},
		},
		{
			name: "query error",
			cfg: input{
				client: &fakeGraphQLClient{
					err: fmt.Errorf("some error"),
					queryResult: &getContextFileQuery{
						ContextFileJSON: contextFile(cluster, testCert),
					},
				},
			},
			expected: expected{
				expectErr: true,
			},
		},
		{
			name: "bad base 64 payload",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getContextFileQuery{
						ContextFileJSON: "bad-base64-format",
					},
				},
			},
			expected: expected{
				expectErr: true,
			},
		},
		{
			name: "no cert",
			cfg: input{
				client: &fakeGraphQLClient{
					queryResult: &getContextFileQuery{
						ContextFileJSON: contextFile("incorrect-cluster", nil),
					},
				},
			},
			expected: expected{
				expectErr: true,
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			uc := &userClient{
				client: tc.cfg.client,
			}
			result, err := uc.GetClusterCertificate(ctx, cluster, "")
			if tc.expected.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expected.cert, result)
		})
	}
}

func TestGetRegistryCredentials(t *testing.T) {
	ctx := context.Background()
	type input struct {
		client *fakeGraphQLClient
		host   string
	}
	type expected struct {
		expectErr       error
		authConfig      dockertypes.AuthConfig
		shouldExpectErr bool
	}
	testCases := []struct {
		cfg      input
		name     string
		expected expected
	}{
		{
			name: "happy path",
			cfg: input{
				host: "1.1.1.1",
				client: &fakeGraphQLClient{
					queryResult: &getRegistryCredentialsQuery{
						RegistryCredentials: registryCredsQuery{
							Username:      "user",
							Password:      "pass",
							Serveraddress: "1.1.1.1",
							Registrytoken: "token1",
							Identitytoken: "token2",
						},
					},
				},
			},
			expected: expected{
				authConfig: dockertypes.AuthConfig{
					Username:      "user",
					Password:      "pass",
					ServerAddress: "1.1.1.1",
					RegistryToken: "token1",
					IdentityToken: "token2",
				},
			},
		},
		{
			name: "fail with docker's not found when old backend",
			cfg: input{
				host: "1.1.1.1",
				client: &fakeGraphQLClient{
					err: errors.New("Cannot query field \"registryCredentials\" on type \"Query\""),
				},
			},
			expected: expected{
				shouldExpectErr: true,
				expectErr:       dockercredentials.NewErrCredentialsNotFound(),
				authConfig:      dockertypes.AuthConfig{},
			},
		},
		{
			name: "fail with docker's not found when not found",
			cfg: input{
				host: "1.1.1.1",
				client: &fakeGraphQLClient{
					err: errors.New("not-found"),
				},
			},
			expected: expected{
				shouldExpectErr: true,
				expectErr:       dockercredentials.NewErrCredentialsNotFound(),
				authConfig:      dockertypes.AuthConfig{},
			},
		},
		{
			name: "fails with other errors",
			cfg: input{
				host: "1.1.1.1",
				client: &fakeGraphQLClient{
					err: errors.New("this is my error. There are many like it but this one is mine"),
				},
			},
			expected: expected{
				shouldExpectErr: true,
				expectErr:       errors.New("this is my error. There are many like it but this one is mine"),
				authConfig:      dockertypes.AuthConfig{},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			uc := &userClient{
				client: tc.cfg.client,
			}
			result, err := uc.GetRegistryCredentials(ctx, tc.cfg.host)
			if tc.expected.shouldExpectErr {
				assert.Error(t, err)
				if tc.expected.expectErr != nil {
					assert.Equal(t, tc.expected.expectErr, err)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expected.authConfig, result)
		})
	}
}
