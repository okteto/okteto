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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

//Client implementation to connect to Okteto API
type OktetoClient struct {
	client *graphql.Client

	namespace types.NamespaceInterface
	user      types.UserInterface
	preview   types.PreviewInterface
}

type OktetoClientProvider struct{}

func NewOktetoClientProvider() *OktetoClientProvider {
	return &OktetoClientProvider{}
}

func (*OktetoClientProvider) Provide() (types.OktetoInterface, error) {
	c, err := NewOktetoClient()
	if err != nil {
		return nil, err
	}
	return c, err
}

//NewClient creates a new client to connect with Okteto API
func NewOktetoClient() (*OktetoClient, error) {
	token := Context().Token
	if token == "" {
		return nil, fmt.Errorf(oktetoErrors.ErrNotLogged, Context().Name)
	}
	u, err := parseOktetoURL(Context().Name)
	if err != nil {
		return nil, err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token,
			TokenType: "Bearer"},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	return newOktetoClientFromGraphqlClient(u, httpClient)
}

//NewClient creates a new client to connect with Okteto API
func NewOktetoClientFromUrlAndToken(url, token string) (*OktetoClient, error) {
	u, err := parseOktetoURL(url)
	if err != nil {
		return nil, err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token,
			TokenType: "Bearer"},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	return newOktetoClientFromGraphqlClient(u, httpClient)
}

//NewClient creates a new client to connect with Okteto API
func NewOktetoClientFromUrl(url string) (*OktetoClient, error) {
	u, err := parseOktetoURL(url)
	if err != nil {
		return nil, err
	}

	httpClient := oauth2.NewClient(context.Background(), nil)

	return newOktetoClientFromGraphqlClient(u, httpClient)
}

func newOktetoClientFromGraphqlClient(url string, httpClient *http.Client) (*OktetoClient, error) {
	c := &OktetoClient{
		client: graphql.NewClient(url, httpClient),
	}
	c.namespace = newNamespaceClient(c.client)
	c.preview = newPreviewClient(c.client)
	c.user = newUserClient(c.client)
	return c, nil
}

func parseOktetoURL(u string) (string, error) {
	if u == "" {
		return "", fmt.Errorf("the okteto URL is not set")
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
		parsed.Host = parsed.Path
	}

	parsed.Path = "graphql"
	return parsed.String(), nil
}

func translateAPIErr(err error) error {
	e := strings.TrimPrefix(err.Error(), "graphql: ")
	switch e {
	case "not-authorized":
		return fmt.Errorf(oktetoErrors.ErrNotLogged, Context().Name)
	case "namespace-quota-exceeded":
		return fmt.Errorf("you have exceeded your namespace quota. Contact us at hello@okteto.com to learn more")
	case "namespace-quota-exceeded-onpremises":
		return fmt.Errorf("you have exceeded your namespace quota, please contact your administrator to increase it")
	case "users-limit-exceeded":
		return fmt.Errorf("license limit exceeded. Contact your administrator to update your license and try again")
	case "internal-server-error":
		return fmt.Errorf("server temporarily unavailable, please try again")
	case "non-200 OK status code: 401 Unauthorized body: \"\"":
		return fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")

	default:
		oktetoLog.Infof("Unrecognized API error: %s", err)
		return fmt.Errorf(e)
	}

}

func isAPITransientErr(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case
		strings.Contains(err.Error(), "can't assign requested address"),
		strings.Contains(err.Error(), "command exited without exit status or exit signal"),
		strings.Contains(err.Error(), "connection refused"),
		strings.Contains(err.Error(), "connection reset by peer"),
		strings.Contains(err.Error(), "client connection lost"),
		strings.Contains(err.Error(), "nodename nor servname provided, or not known"),
		strings.Contains(err.Error(), "unexpected EOF"),
		strings.Contains(err.Error(), "TLS handshake timeout"),
		strings.Contains(err.Error(), "broken pipe"),
		strings.Contains(err.Error(), "No connection could be made"),
		strings.Contains(err.Error(), "dial tcp: operation was canceled"),
		strings.Contains(err.Error(), "network is unreachable"),
		strings.Contains(err.Error(), "development container has been removed"):
		return true
	default:
		return false
	}

}

// InDevContainer returns true if running in an okteto dev container
func InDevContainer() bool {
	if v, ok := os.LookupEnv(model.OktetoNameEnvVar); ok && v != "" {
		return true
	}

	return false
}

func query(ctx context.Context, query interface{}, variables map[string]interface{}, client *graphql.Client) error {
	err := client.Query(ctx, query, variables)
	if err != nil {
		if isAPITransientErr(err) {
			err = client.Query(ctx, query, variables)
		}
		if err != nil {
			return translateAPIErr(err)
		}
	}
	return nil
}

func mutate(ctx context.Context, mutation interface{}, variables map[string]interface{}, client *graphql.Client) error {
	err := client.Mutate(ctx, mutation, variables)
	if err != nil {
		return translateAPIErr(err)
	}
	return nil
}

// Namespace retrieves the NamespaceClient
func (c *OktetoClient) Namespaces() types.NamespaceInterface {
	return c.namespace
}

// Namespace retrieves the NamespaceClient
func (c *OktetoClient) Previews() types.PreviewInterface {
	return c.preview
}

// User retrieves the UserClient
func (c *OktetoClient) User() types.UserInterface {
	return c.user
}
