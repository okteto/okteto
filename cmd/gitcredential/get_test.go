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

package gitcredential

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

const githubRequest = "protocol=https\nhost=github.com\npath=acme/infra.git\n\n"

// fakeGetter records the request it gets and answers with creds and err
type fakeGetter struct {
	err        error
	host       string
	repository string
	creds      types.GitCredentials
}

func (f *fakeGetter) GetGitCredentials(_ context.Context, host, repository string) (types.GitCredentials, error) {
	f.host = host
	f.repository = repository
	return f.creds, f.err
}

func providerFor(getter credentialsGetter) getterProvider {
	return func() (credentialsGetter, error) {
		return getter, nil
	}
}

func runGet(input string, provider getterProvider) (string, string) {
	var stdout, stderr bytes.Buffer
	get(context.Background(), strings.NewReader(input), &stdout, &stderr, provider)
	return stdout.String(), stderr.String()
}

func TestGet(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		creds          types.GitCredentials
		expectedStdout string
		expectedStderr string
	}{
		{
			name:  "token with an expiry",
			input: githubRequest,
			creds: types.GitCredentials{
				Username:  "x-access-token",
				Password:  "token",
				ExpiresAt: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
			},
			expectedStdout: "username=x-access-token\npassword=token\npassword_expiry_utc=1794564000\n",
		},
		{
			name:           "token without an expiry",
			input:          githubRequest,
			creds:          types.GitCredentials{Username: "x-access-token", Password: "token"},
			expectedStdout: "username=x-access-token\npassword=token\n",
		},
		{
			name:           "token without a username",
			input:          githubRequest,
			creds:          types.GitCredentials{Password: "token"},
			expectedStdout: "username=x-access-token\npassword=token\n",
		},
		{
			name:           "no token prints the API's reason",
			input:          githubRequest,
			creds:          types.GitCredentials{Reason: "link your GitHub account in Okteto, in Settings → Integrations"},
			expectedStderr: "okteto: no GitHub credentials for acme/infra: link your GitHub account in Okteto, in Settings → Integrations\n",
		},
		{
			name:           "no token and no reason",
			input:          githubRequest,
			expectedStderr: "okteto: no GitHub credentials for acme/infra: the Okteto API returned no token\n",
		},
		{
			name:           "reason with control characters stays on one line",
			input:          githubRequest,
			creds:          types.GitCredentials{Reason: "ask an admin\n\tto add it\x1b[31m"},
			expectedStderr: "okteto: no GitHub credentials for acme/infra: ask an admin to add it [31m\n",
		},
		{
			name:           "password with a newline",
			input:          githubRequest,
			creds:          types.GitCredentials{Username: "x-access-token", Password: "token\nusername=evil"},
			expectedStderr: "okteto: no GitHub credentials for acme/infra: the Okteto API returned a credential with a newline or NUL character\n",
		},
		{
			name:           "request without a path",
			input:          "protocol=https\nhost=github.com\n\n",
			creds:          types.GitCredentials{Password: "token"},
			expectedStderr: "okteto: no GitHub credentials for github.com: the request from Git has no repository path, set credential.https://github.com.useHttpPath to true\n",
		},
		{
			name:           "path with a terminal escape sequence",
			input:          "protocol=https\nhost=github.com\npath=acme/infra\x1b[31m.git\n\n",
			creds:          types.GitCredentials{Password: "token"},
			expectedStderr: "okteto: no GitHub credentials for github.com: the path in the request from Git isn't a GitHub repository\n",
		},
		{
			name:  "another host gets nothing",
			input: "protocol=https\nhost=gitlab.com\npath=acme/infra.git\n\n",
			creds: types.GitCredentials{Password: "token"},
		},
		{
			name:  "plain http gets nothing",
			input: "protocol=http\nhost=github.com\npath=acme/infra.git\n\n",
			creds: types.GitCredentials{Password: "token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := runGet(tt.input, providerFor(&fakeGetter{creds: tt.creds}))

			require.Equal(t, tt.expectedStdout, stdout)
			require.Equal(t, tt.expectedStderr, stderr)
		})
	}
}

func TestGet_AsksForTheRepositoryOnGitHub(t *testing.T) {
	getter := &fakeGetter{creds: types.GitCredentials{Password: "token"}}

	runGet("protocol=https\nhost=GitHub.com\npath=acme/infra.git/info/lfs\n\n", providerFor(getter))

	require.Equal(t, "github.com", getter.host)
	require.Equal(t, "acme/infra", getter.repository)
}

func TestGet_APIErrorPrintsOneLine(t *testing.T) {
	getter := &fakeGetter{err: errors.New("graphql: internal-server-error")}

	stdout, stderr := runGet(githubRequest, providerFor(getter))

	require.Empty(t, stdout)
	require.Equal(t, "okteto: no GitHub credentials for acme/infra: graphql: internal-server-error\n", stderr)
}

func TestGet_ProviderErrorPrintsOneLine(t *testing.T) {
	provider := func() (credentialsGetter, error) {
		return nil, errTokenNotSet
	}

	stdout, stderr := runGet(githubRequest, provider)

	require.Empty(t, stdout)
	require.Equal(t, "okteto: no GitHub credentials for acme/infra: OKTETO_TOKEN is not set\n", stderr)
}

func TestGet_OtherHostsNeverBuildTheClient(t *testing.T) {
	called := false
	provider := func() (credentialsGetter, error) {
		called = true
		return nil, errTokenNotSet
	}

	stdout, stderr := runGet("protocol=https\nhost=gitlab.com\npath=acme/infra.git\n\n", provider)

	require.False(t, called)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
}
