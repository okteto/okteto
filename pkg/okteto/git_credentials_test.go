// Copyright 2026 The Okteto Authors
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
	"errors"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestGetGitCredentials(t *testing.T) {
	errOther := errors.New("graphql: internal-server-error")

	tests := []struct {
		expectedErr error
		client      *fakeGraphQLClient
		name        string
		expected    types.GitCredentials
	}{
		{
			name: "token with an expiry",
			client: &fakeGraphQLClient{
				queryResult: &getGitCredentialsQuery{
					GitCredentials: gitCredentialsQuery{
						Username:  "x-access-token",
						Password:  "token",
						ExpiresAt: "2026-11-13T10:00:00Z",
					},
				},
			},
			expected: types.GitCredentials{
				Username:  "x-access-token",
				Password:  "token",
				ExpiresAt: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "token without an expiry",
			client: &fakeGraphQLClient{
				queryResult: &getGitCredentialsQuery{
					GitCredentials: gitCredentialsQuery{
						Username: "x-access-token",
						Password: "token",
					},
				},
			},
			expected: types.GitCredentials{
				Username: "x-access-token",
				Password: "token",
			},
		},
		{
			name: "token with an expiry that can't be read keeps the token",
			client: &fakeGraphQLClient{
				queryResult: &getGitCredentialsQuery{
					GitCredentials: gitCredentialsQuery{
						Username:  "x-access-token",
						Password:  "token",
						ExpiresAt: "tomorrow",
					},
				},
			},
			expected: types.GitCredentials{
				Username: "x-access-token",
				Password: "token",
			},
		},
		{
			name: "no token with a reason",
			client: &fakeGraphQLClient{
				queryResult: &getGitCredentialsQuery{
					GitCredentials: gitCredentialsQuery{
						Reason: "link your GitHub account in Okteto, in Settings → Integrations",
					},
				},
			},
			expected: types.GitCredentials{
				Reason: "link your GitHub account in Okteto, in Settings → Integrations",
			},
		},
		{
			name: "instance older than the query",
			client: &fakeGraphQLClient{
				err: errors.New(`Cannot query field "gitCredentials" on type "Query".`),
			},
			expectedErr: ErrGitCredentialsNotSupported,
		},
		{
			name: "token rejected by the API",
			client: &fakeGraphQLClient{
				err: errors.New("graphql: not-authorized"),
			},
			expectedErr: ErrGitCredentialsNotAuthorized,
		},
		{
			name: "token rejected with a 401",
			client: &fakeGraphQLClient{
				err: errors.New(`non-200 OK status code: 401 Unauthorized body: "not-authorized: token is expired\n"`),
			},
			expectedErr: ErrGitCredentialsNotAuthorized,
		},
		{
			name: "other API error",
			client: &fakeGraphQLClient{
				err: errOther,
			},
			expectedErr: errOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newUserClient(tt.client, nil, "")

			creds, err := uc.GetGitCredentials(context.Background(), "github.com", "acme/infra")

			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expected, creds)
		})
	}
}

func TestGetGitCredentials_DropsTheBodyOfAnErrorResponse(t *testing.T) {
	client := &fakeGraphQLClient{
		err: errors.New(`non-200 OK status code: 502 Bad Gateway body: "<html><body><h1>502 Bad Gateway</h1></body></html>\n"`),
	}
	uc := newUserClient(client, nil, "")

	_, err := uc.GetGitCredentials(context.Background(), "github.com", "acme/infra")

	require.EqualError(t, err, "non-200 OK status code: 502 Bad Gateway")
}
