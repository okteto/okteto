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
	"io/ioutil"
	"net/url"
	"regexp"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

var reg = regexp.MustCompile("[^A-Za-z0-9]+")

// Token contains the auth token and the URL it belongs to
type Token struct {
	URL             string `json:"URL"`
	Buildkit        string `json:"Buildkit"`
	Registry        string `json:"Registry"`
	ID              string `json:"ID"`
	Username        string `json:"Username"`
	Token           string `json:"Token"`
	MachineID       string `json:"MachineID"`
	GlobalNamespace string `json:"GlobalNamespace"`
}

// User contains the auth information of the logged in user
type User struct {
	Name            string
	Email           string
	ExternalID      string
	Token           string
	ID              string
	New             bool
	Buildkit        string
	Registry        string
	Certificate     string
	GlobalNamespace string
}

type u struct {
	Auth User
}

type q struct {
	User User
}

// AuthWithToken authenticates in okteto with the provided token
func AuthWithToken(ctx context.Context, u, token string) (*User, error) {
	url, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}

	client, err := getClient(url.String())
	if err != nil {
		return nil, err
	}

	user, err := queryUser(ctx, client, token)
	if err != nil {
		log.Infof("failed to query the user with the existing token: %s", err)
		return nil, fmt.Errorf("invalid API token")
	}

	return &user.User, nil
}

// Auth authenticates in okteto with an OAuth code
func Auth(ctx context.Context, code, url string) (*User, error) {
	client, err := getClient(url)
	if err != nil {
		return nil, err
	}

	user, err := authUser(ctx, client, code)
	if err != nil {
		log.Infof("authentication error: %s", err)
		return nil, fmt.Errorf("authentication error, please try again")
	}

	return &user.Auth, nil
}

func queryUser(ctx context.Context, client *graphql.Client, token string) (*q, error) {
	var user q
	q := `query {
		user {
			id,name,email,externalID,token,new,registry,buildkit,certificate,globalNamespace
		}}`

	req := getRequest(q, token)

	if err := client.Run(ctx, req, &user); err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"globalNamespace\" on type \"me\"") {
			return deprecatedQueryUser(ctx, client, token)
		}
		return nil, err
	}

	return &user, nil
}

//TODO: remove when all users are in Okteto Enterprise which supports globalNamespace
func deprecatedQueryUser(ctx context.Context, client *graphql.Client, token string) (*q, error) {
	var user q
	q := `query {
		user {
			id,name,email,externalID,token,new,registry,buildkit,certificate
		}}`

	req := getRequest(q, token)

	if err := client.Run(ctx, req, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func authUser(ctx context.Context, client *graphql.Client, code string) (*u, error) {
	var user u
	q := fmt.Sprintf(`mutation {
		auth(code: "%s", source: "cli") {
			id,name,email,externalID,token,new,registry,buildkit,certificate,globalNamespace
		}}`, code)

	req := graphql.NewRequest(q)
	if err := client.Run(ctx, req, &user); err != nil {
		if strings.Contains(err.Error(), "Cannot query field \"globalNamespace\" on type \"me\"") {
			return deprecatedAuthUser(ctx, client, code)
		}
		return nil, err
	}

	return &user, nil
}

//TODO: remove when all users are in Okteto Enterprise which supports globalNamespace
func deprecatedAuthUser(ctx context.Context, client *graphql.Client, code string) (*u, error) {
	var user u
	q := fmt.Sprintf(`mutation {
		auth(code: "%s", source: "cli") {
			id,name,email,externalID,token,new,registry,buildkit,certificate
		}}`, code)

	req := graphql.NewRequest(q)
	if err := client.Run(ctx, req, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func getTokenFromOktetoHome() (*Token, error) {
	p := config.GetTokenPathDeprecated()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	currentToken := &Token{}
	if err := json.Unmarshal(b, currentToken); err != nil {
		return nil, err
	}

	return currentToken, nil
}

//IsAuthenticated returns if the user is authenticated
func IsAuthenticated() bool {
	t, err := getTokenFromOktetoHome()
	if err != nil {
		log.Infof("error getting okteto token: %s", err)
		return false
	}
	return t.Token != ""
}
