// Copyright 2021 The Okteto Authors
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
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
)

var reg = regexp.MustCompile("[^A-Za-z0-9]+")

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

// Auth authenticates in okteto with an OAuth code
func Auth(ctx context.Context, code, url string) (*types.User, error) {
	oktetoClient, err := NewOktetoClientFromUrl(url)
	if err != nil {
		return nil, err
	}

	user, err := oktetoClient.authUser(ctx, code)
	if err != nil {
		log.Infof("authentication error: %s", err)
		return nil, fmt.Errorf("authentication error, please try again")
	}

	return user, nil
}

func (c *OktetoClient) authUser(ctx context.Context, code string) (*types.User, error) {
	var mutation struct {
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
		} `graphql:"auth(code: $code, source: $source)"`
	}

	queryVariables := map[string]interface{}{
		"code":   graphql.String(code),
		"source": graphql.String("cli"),
	}

	err := c.Mutate(ctx, &mutation, queryVariables)
	if err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"globalNamespace\" on type \"me\"") {
			return c.deprecatedAuthUser(ctx, code)
		}
		if strings.Contains(err.Error(), "Cannot query field \"telemetryEnabled\" on type \"me\"") {
			return c.deprecatedAuthUser(ctx, code)
		}
		return nil, err
	}

	globalNamespace := getGlobalNamespace(string(mutation.User.GlobalNamespace))
	analytics := bool(mutation.User.Analytics)

	user := &types.User{
		ID:              string(mutation.User.Id),
		Name:            string(mutation.User.Name),
		Namespace:       string(mutation.User.Namespace),
		Email:           string(mutation.User.Email),
		ExternalID:      string(mutation.User.ExternalID),
		Token:           string(mutation.User.Token),
		New:             bool(mutation.User.New),
		Registry:        string(mutation.User.Registry),
		Buildkit:        string(mutation.User.Buildkit),
		Certificate:     string(mutation.User.Certificate),
		GlobalNamespace: globalNamespace,
		Analytics:       analytics,
	}

	return user, nil
}

func (c *OktetoClient) deprecatedAuthUser(ctx context.Context, code string) (*types.User, error) {
	var mutation struct {
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
		} `graphql:"auth(code: $code, source: $source)"`
	}

	queryVariables := map[string]interface{}{
		"code":   graphql.String(code),
		"source": graphql.String("cli"),
	}

	err := c.Mutate(ctx, &mutation, queryVariables)
	if err != nil {
		return nil, err
	}

	user := &types.User{
		ID:              string(mutation.User.Id),
		Name:            string(mutation.User.Name),
		Namespace:       string(mutation.User.Namespace),
		Email:           string(mutation.User.Email),
		ExternalID:      string(mutation.User.ExternalID),
		Token:           string(mutation.User.Token),
		New:             bool(mutation.User.New),
		Registry:        string(mutation.User.Registry),
		Buildkit:        string(mutation.User.Buildkit),
		Certificate:     string(mutation.User.Certificate),
		GlobalNamespace: DefaultGlobalNamespace,
		Analytics:       true,
	}

	return user, nil
}

func getTokenFromOktetoHome() (*Token, error) {
	p := config.GetTokenPathDeprecated()

	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	currentToken := &Token{}
	if err := json.Unmarshal(b, currentToken); err != nil {
		return nil, err
	}

	return currentToken, nil
}

func getGlobalNamespace(g string) string {
	if g == "" {
		return DefaultGlobalNamespace
	}
	return g
}
