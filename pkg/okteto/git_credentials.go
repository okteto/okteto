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
	"strings"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

var (
	// ErrGitCredentialsNotSupported is returned when the Okteto instance is older than the gitCredentials query
	ErrGitCredentialsNotSupported = errors.New("this Okteto instance doesn't give Git credentials")

	// ErrGitCredentialsNotAuthorized is returned when the Okteto API rejects the token
	ErrGitCredentialsNotAuthorized = errors.New("the Okteto token is not valid or has expired")
)

type getGitCredentialsQuery struct {
	GitCredentials gitCredentialsQuery `graphql:"gitCredentials(host: $host, repository: $repository)"`
}

type gitCredentialsQuery struct {
	Username  graphql.String
	Password  graphql.String
	ExpiresAt graphql.String `graphql:"expiresAt"`
	Reason    graphql.String
}

// GetGitCredentials returns the credentials Okteto gives for a repository ("owner/repo") of a Git host.
// Without credentials, the API explains why in Reason.
// Its errors don't go through translateAPIErr: the Git credential helper calls it without
// an Okteto context, and translateAPIErr reads the current one.
func (c *userClient) GetGitCredentials(ctx context.Context, host, repository string) (types.GitCredentials, error) {
	var q getGitCredentialsQuery
	vars := map[string]interface{}{
		"host":       graphql.String(host),
		"repository": graphql.String(repository),
	}
	err := c.client.Query(ctx, &q, vars)
	if isAPITransientErr(err) {
		err = c.client.Query(ctx, &q, vars)
	}
	if err != nil {
		return types.GitCredentials{}, translateGitCredentialsErr(err)
	}

	creds := types.GitCredentials{
		Username: string(q.GitCredentials.Username),
		Password: string(q.GitCredentials.Password),
		Reason:   string(q.GitCredentials.Reason),
	}
	// Git uses the token until it fails when it doesn't know the expiry, so an expiry
	// that can't be read is dropped instead of the token
	if expiresAt := string(q.GitCredentials.ExpiresAt); expiresAt != "" {
		t, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			oktetoLog.Infof("ignoring the expiry of the Git credentials, %q is not RFC 3339: %s", expiresAt, err)
		} else {
			creds.ExpiresAt = t
		}
	}
	return creds, nil
}

func translateGitCredentialsErr(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, `Cannot query field "gitCredentials"`):
		return ErrGitCredentialsNotSupported
	case strings.Contains(msg, "not-authorized"), strings.Contains(msg, "401 Unauthorized"):
		return ErrGitCredentialsNotAuthorized
	default:
		return withoutResponseBody(err)
	}
}

// withoutResponseBody drops the response body that the GraphQL client adds to the error of
// a non-200 answer, such as an ingress's HTML error page: Git prints the helper's error in
// the deploy logs, and the status says enough
func withoutResponseBody(err error) error {
	msg := err.Error()
	if i := strings.Index(msg, " body: "); i >= 0 {
		return errors.New(msg[:i])
	}
	return err
}
