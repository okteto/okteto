package okteto

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

const (
	tokenFile = ".token.json"

	// CloudURL is the default URL of okteto
	CloudURL = "https://cloud.okteto.com"
)

// Token contains the auth token and the URL it belongs to
type Token struct {
	Token string
	URL   string
	ID    string
}

// User contains the auth information of the logged in user
type User struct {
	Name     string
	Email    string
	GithubID string
	Token    string
	ID       string
	New      bool
}

var token *Token

// Auth authenticates in okteto with a github OAuth code
func Auth(ctx context.Context, code, url string) (*User, error) {
	client, err := getClient(url)
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf(`
				mutation {
					auth(code: "%s", source: "cli") {
					  id,name,email,githubID,token,new
					}
				  }`, code)

	req := graphql.NewRequest(q)

	type u struct {
		Auth User
	}

	var user u
	if err := client.Run(ctx, req, &user); err != nil {
		return nil, fmt.Errorf("unauthorized request: %s", err)
	}

	if len(user.Auth.GithubID) == 0 || len(user.Auth.Token) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	if err := saveToken(user.Auth.ID, user.Auth.Token, url); err != nil {
		return nil, err
	}

	return &user.Auth, nil
}

func getTokenFromEnv() (*Token, error) {
	log.Info("using token from environment")
	t := &Token{Token: os.Getenv("OKTETO_TOKEN")}
	u := os.Getenv("OKTETO_URL")
	if len(u) == 0 {
		u = CloudURL
	}

	p, err := url.Parse(u)
	if err != nil {
		return t, err
	}

	t.URL = p.String()

	return t, nil
}

func getToken() (*Token, error) {
	if token == nil {
		if len(os.Getenv("OKTETO_TOKEN")) > 0 {
			return getTokenFromEnv()
		}

		p := filepath.Join(config.GetHome(), tokenFile)

		b, err := ioutil.ReadFile(p)
		if err != nil {
			return nil, err
		}

		token = &Token{}
		if err := json.Unmarshal(b, token); err != nil {
			return nil, err
		}
	}

	return token, nil
}

// GetUserID returns the userID of the authenticated user
func GetUserID() string {
	t, err := getToken()
	if err != nil {
		return ""
	}

	return t.ID
}

// GetURL returns the URL of the authenticated user
func GetURL() string {
	t, err := getToken()
	if err != nil {
		return "na"
	}

	return t.URL
}

// GetURLWithUnderscore returns the URL of the authenticated user with underscores
func GetURLWithUnderscore() string {
	u, _ := url.Parse(GetURL())
	return strings.ReplaceAll(u.Host, ".", "_")
}

func saveToken(id, token, url string) error {
	t := Token{Token: token, URL: url, ID: id}
	marshalled, err := json.Marshal(t)
	if err != nil {
		log.Infof("failed to marshal token: %s", err)
		return fmt.Errorf("Failed to generate your auth token")
	}

	p := filepath.Join(config.GetHome(), tokenFile)
	log.Debugf("saving token at %s", p)
	if err := ioutil.WriteFile(p, marshalled, 400); err != nil {
		return fmt.Errorf("couldn't save authentication token: %s", err)
	}

	return nil
}
