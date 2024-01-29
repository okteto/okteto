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
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	okerrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	x509err := fmt.Errorf("x509 error")
	licerr := fmt.Errorf("non-200 OK status code: 423")
	type input struct {
		client fakeGraphQLClient
	}
	type expected struct {
		user *types.User
		err  error
	}
	tests := []struct {
		expected expected
		input    input
		name     string
	}{
		{
			name: "error authenticating",
			input: input{
				client: fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				user: nil,
				err:  assert.AnError,
			},
		},
		{
			name: "github email not verified",
			input: input{
				client: fakeGraphQLClient{
					err: okerrors.ErrGitHubNotVerifiedEmail,
				},
			},
			expected: expected{
				user: nil,
				err:  errGitHubNotVerifiedEmail,
			},
		},
		{
			name: "missing business email",
			input: input{
				client: fakeGraphQLClient{
					err: ErrGithubMissingBusinessEmail,
				},
			},
			expected: expected{
				user: nil,
				err:  ErrGithubMissingBusinessEmail,
			},
		},
		{
			name: "x509",
			input: input{
				client: fakeGraphQLClient{
					err: x509err,
				},
			},
			expected: expected{
				user: nil,
				err:  x509err,
			},
		},
		{
			name: "return a proper user",
			input: input{
				client: fakeGraphQLClient{
					mutationResult: &authMutationStruct{
						Response: userMutation{},
					},
				},
			},
			expected: expected{
				user: &types.User{
					GlobalNamespace: constants.DefaultGlobalNamespace,
				},
				err: nil,
			},
		},
		{
			name: "trial expired",
			input: input{
				client: fakeGraphQLClient{
					err: licerr,
				},
			},
			expected: expected{
				err: okerrors.ErrInvalidLicense,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Client{
				client: tt.input.client,
			}
			u, err := c.Auth(context.Background(), "")
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.user, u)
		})
	}
}

func TestAuthGQLCall(t *testing.T) {
	type input struct {
		client fakeGraphQLClient
	}
	type expected struct {
		user *types.User
		err  error
	}
	tests := []struct {
		input    input
		expected expected
		name     string
	}{
		{
			name: "error",
			input: input{
				client: fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "return user",
			input: input{
				client: fakeGraphQLClient{
					mutationResult: &authMutationStruct{
						Response: userMutation{
							Id:              "test",
							Name:            "test",
							Namespace:       "test",
							Email:           "test",
							ExternalID:      "test",
							Token:           "test",
							New:             true,
							Registry:        "test",
							Buildkit:        "test",
							Certificate:     "test",
							GlobalNamespace: "test",
							Analytics:       false,
						},
					},
				},
			},
			expected: expected{
				user: &types.User{
					Name:            "test",
					Namespace:       "test",
					Email:           "test",
					ExternalID:      "test",
					Token:           "test",
					New:             true,
					Registry:        "test",
					Buildkit:        "test",
					Certificate:     "test",
					ID:              "test",
					GlobalNamespace: "test",
					Analytics:       false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Client{
				client: tt.input.client,
			}
			u, err := c.authUser(context.Background(), "")
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.user, u)
		})
	}
}

func TestDeprecatedAuth(t *testing.T) {
	type input struct {
		client fakeGraphQLClient
	}
	type expected struct {
		user *types.User
		err  error
	}
	tests := []struct {
		expected expected
		input    input
		name     string
	}{
		{
			name: "error",
			input: input{
				client: fakeGraphQLClient{
					err: assert.AnError,
				},
			},
			expected: expected{
				err: assert.AnError,
			},
		},
		{
			name: "return user",
			input: input{
				client: fakeGraphQLClient{
					mutationResult: &deprecatedAuthMutationStruct{
						Response: deprecatedUserMutation{
							Id:          "test",
							Name:        "test",
							Namespace:   "test",
							Email:       "test",
							ExternalID:  "test",
							Token:       "test",
							New:         true,
							Registry:    "test",
							Buildkit:    "test",
							Certificate: "test",
						},
					},
				},
			},
			expected: expected{
				user: &types.User{
					Name:            "test",
					Namespace:       "test",
					Email:           "test",
					ExternalID:      "test",
					Token:           "test",
					New:             true,
					Registry:        "test",
					Buildkit:        "test",
					Certificate:     "test",
					ID:              "test",
					GlobalNamespace: constants.DefaultGlobalNamespace,
					Analytics:       true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Client{
				client: tt.input.client,
			}
			u, err := c.deprecatedAuthUser(context.Background(), "")
			assert.ErrorIs(t, err, tt.expected.err)
			assert.Equal(t, tt.expected.user, u)
		})
	}
}
