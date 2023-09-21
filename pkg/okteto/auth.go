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
	"errors"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

const cliSource = "cli"

// Token contains the auth token and the URL it belongs to
type Token struct {
	URL       string `json:"URL"`
	Buildkit  string `json:"Buildkit"`
	Registry  string `json:"Registry"`
	ID        string `json:"ID"`
	Username  string `json:"Username"`
	Token     string `json:"Token"`
	MachineID string `json:"MachineID"`
}

type authenticationErr struct {
	E error
}

func (*authenticationErr) Error() string {
	return "authentication error, please try again"
}
func (e *authenticationErr) Unwrap() error {
	return e.E
}

func newAuthenticationErr(err error) *authenticationErr {
	return &authenticationErr{
		E: err,
	}
}

var errGitHubNotVerifiedEmail = errors.New("your GitHub account doesn't have a verified primary email address. Please check your GitHub account email settings and try again")

type authMutationStruct struct {
	Response userMutation `graphql:"auth(code: $code, source: $source)"`
}

type deprecatedAuthMutationStruct struct {
	Response deprecatedUserMutation `graphql:"auth(code: $code, source: $source)"`
}

type userMutation struct {
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

type deprecatedUserMutation struct {
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

// Auth authenticates in okteto with an OAuth code
func (c *OktetoClient) Auth(ctx context.Context, code string) (*types.User, error) {
	user, err := c.authUser(ctx, code)
	if err != nil {
		oktetoLog.Infof("authentication error: %s", err)
		if oktetoErrors.IsErrGitHubNotVerifiedEmail(err) {
			return nil, errGitHubNotVerifiedEmail
		}
		// This error is sent at the mutation with Metadata. Our current client for GraphQL does not support this kind of errors,
		// so the information regarding metada is lost here. Message is still comunicated so we can check the error
		// https://github.com/okteto/okteto/issues/2926
		if IsErrGithubMissingBusinessEmail(err) {
			return nil, err
		}

		// If there is a TLS error, return the raw error
		if err != nil && oktetoErrors.IsX509(err) {
			return nil, err
		}

		if isAPILicenseError(err) {
			return nil, err
		}

		err := newAuthenticationErr(err)
		return nil, err
	}

	return user, nil
}

func (c *OktetoClient) authUser(ctx context.Context, code string) (*types.User, error) {
	var mutation authMutationStruct

	queryVariables := map[string]interface{}{
		"code":   graphql.String(code),
		"source": graphql.String(cliSource),
	}

	err := mutate(ctx, &mutation, queryVariables, c.client)
	if err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"globalNamespace\" on type \"me\"") {
			return c.deprecatedAuthUser(ctx, code)
		}
		if strings.Contains(err.Error(), "Cannot query field \"telemetryEnabled\" on type \"me\"") {
			return c.deprecatedAuthUser(ctx, code)
		}
		return nil, err
	}

	globalNamespace := getGlobalNamespace(string(mutation.Response.GlobalNamespace))
	analytics := bool(mutation.Response.Analytics)

	user := &types.User{
		ID:              string(mutation.Response.Id),
		Name:            string(mutation.Response.Name),
		Namespace:       string(mutation.Response.Namespace),
		Email:           string(mutation.Response.Email),
		ExternalID:      string(mutation.Response.ExternalID),
		Token:           string(mutation.Response.Token),
		New:             bool(mutation.Response.New),
		Registry:        string(mutation.Response.Registry),
		Buildkit:        string(mutation.Response.Buildkit),
		Certificate:     string(mutation.Response.Certificate),
		GlobalNamespace: globalNamespace,
		Analytics:       analytics,
	}

	return user, nil
}

// TODO: Remove this code when okteto char 0.10.8 is deprecated
func (c *OktetoClient) deprecatedAuthUser(ctx context.Context, code string) (*types.User, error) {
	var mutation deprecatedAuthMutationStruct
	queryVariables := map[string]interface{}{
		"code":   graphql.String(code),
		"source": graphql.String(cliSource),
	}

	err := mutate(ctx, &mutation, queryVariables, c.client)
	if err != nil {
		return nil, err
	}

	user := &types.User{
		ID:              string(mutation.Response.Id),
		Name:            string(mutation.Response.Name),
		Namespace:       string(mutation.Response.Namespace),
		Email:           string(mutation.Response.Email),
		ExternalID:      string(mutation.Response.ExternalID),
		Token:           string(mutation.Response.Token),
		New:             bool(mutation.Response.New),
		Registry:        string(mutation.Response.Registry),
		Buildkit:        string(mutation.Response.Buildkit),
		Certificate:     string(mutation.Response.Certificate),
		GlobalNamespace: constants.DefaultGlobalNamespace,
		Analytics:       true,
	}

	return user, nil
}

func getGlobalNamespace(g string) string {
	if g == "" {
		return constants.DefaultGlobalNamespace
	}
	return g
}
