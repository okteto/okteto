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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
)

const (
	tokenFile = ".token.json"
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

// User contains the auth information of the logged in user
type User struct {
	Name        string
	Email       string
	ExternalID  string
	Token       string
	ID          string
	New         bool
	Buildkit    string
	Registry    string
	Certificate string
}

type u struct {
	Auth User
}

type q struct {
	User User
}

var currentToken *Token

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

	if err := saveAuthData(&user.User, url.String()); err != nil {
		log.Infof("failed to save the login data: %s", err)
		return nil, fmt.Errorf("failed to save the login data locally")
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

	if err := saveAuthData(&user.Auth, url); err != nil {
		log.Infof("failed to save the auth data: %s", err)
		return nil, fmt.Errorf("failed to save your auth info locally, please try again")
	}

	return &user.Auth, nil

}

func saveAuthData(user *User, url string) error {
	if user.ExternalID == "" || user.Token == "" {
		return fmt.Errorf("empty response")
	}

	if err := saveToken(user.ID, user.ExternalID, user.Token, url, user.Registry, user.Buildkit); err != nil {
		return err
	}

	d, err := base64.StdEncoding.DecodeString(user.Certificate)
	if err != nil {
		return fmt.Errorf("certificate decoding error: %w", err)
	}

	return ioutil.WriteFile(GetCertificatePath(), d, 0600)
}

func queryUser(ctx context.Context, client *graphql.Client, token string) (*q, error) {
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
			id,name,email,externalID,token,new,registry,buildkit,certificate
		}}`, code)

	req := graphql.NewRequest(q)
	if err := client.Run(ctx, req, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

//GetToken returns the token of the authenticated user
func GetToken() (*Token, error) {
	if currentToken == nil {
		p := getTokenPath()

		b, err := ioutil.ReadFile(p)
		if err != nil {
			return nil, err
		}

		currentToken = &Token{}
		if err := json.Unmarshal(b, currentToken); err != nil {
			return nil, err
		}
	}

	return currentToken, nil
}

//IsAuthenticated returns if the user is authenticated
func IsAuthenticated() bool {
	t, err := GetToken()
	if err != nil {
		log.Infof("error getting okteto token: %s", err)
		return false
	}
	return t.Token != ""
}

// GetUserID returns the userID of the authenticated user
func GetUserID() string {
	t, err := GetToken()
	if err != nil {
		return ""
	}

	return t.ID
}

// GetUsername returns the username of the authenticated user
func GetUsername() string {
	t, err := GetToken()
	if err != nil {
		return ""
	}

	return t.Username
}

// GetSanitizedUsername returns the username of the authenticated user sanitized to be DNS compatible
func GetSanitizedUsername() string {
	t, err := GetToken()
	if err != nil {
		return ""
	}

	return reg.ReplaceAllString(strings.ToLower(t.Username), "-")
}

// GetMachineID returns the userID of the authenticated user
func GetMachineID() string {
	t, err := GetToken()
	if err != nil {
		return ""
	}

	return t.MachineID
}

// GetURL returns the URL of the authenticated user
func GetURL() string {
	t, err := GetToken()
	if err != nil {
		return "na"
	}

	return t.URL
}

// GetRegistry returns the URL of the registry
func GetRegistry() (string, error) {
	t, err := GetToken()
	if err != nil {
		return "", errors.ErrNotLogged
	}
	if t.Registry == "" {
		if GetURL() == CloudURL {
			return CloudRegistryURL, nil
		}
		return "", errors.ErrNotLogged
	}
	return t.Registry, nil
}

// GetBuildKit returns the URL of the okteto buildkit
func GetBuildKit() (string, error) {
	t, err := GetToken()
	if err != nil {
		return "", errors.ErrNotLogged
	}
	if t.Buildkit == "" {
		if GetURL() == CloudURL {
			return CloudBuildKitURL, nil
		}
		return "", errors.ErrNotLogged
	}
	return t.Buildkit, nil
}

// GetCertificatePath returns the path  to the certificate of the okteto buildkit
func GetCertificatePath() string {
	return filepath.Join(config.GetOktetoHome(), ".ca.crt")
}

func saveToken(id, username, token, url, registry, buildkit string) error {
	t, err := GetToken()
	if err != nil {
		log.Infof("bad token, re-initializing: %s", err)
		t = &Token{}
	}

	t.ID = id
	t.Username = username
	t.Token = token
	t.URL = url
	t.Buildkit = buildkit
	t.Registry = registry
	return save(t)
}

// SaveMachineID updates the token file with the machineID value
func SaveMachineID(machineID string) error {
	t, err := GetToken()
	if err != nil {
		log.Infof("bad token, re-initializing: %s", err)
		t = &Token{}
	}

	t.MachineID = machineID
	return save(t)
}

// SaveID updates the token file with the userID value
func SaveID(userID string) error {
	t, err := GetToken()
	if err != nil {
		log.Infof("bad token, re-initializing: %s", err)
		t = &Token{}
	}

	t.ID = userID
	return save(t)
}

func save(t *Token) error {
	marshalled, err := json.Marshal(t)
	if err != nil {
		log.Infof("failed to marshal token: %s", err)
		return fmt.Errorf("Failed to generate your auth token")
	}

	p := getTokenPath()

	if _, err := os.Stat(p); err == nil {
		err = os.Chmod(p, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change token permissions: %s", err)
		}
	}

	if err := ioutil.WriteFile(p, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save authentication token: %s", err)
	}

	currentToken = nil
	return nil
}

func getTokenPath() string {
	return filepath.Join(config.GetOktetoHome(), tokenFile)
}
