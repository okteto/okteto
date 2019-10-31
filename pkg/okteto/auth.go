package okteto

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

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
	Token     string
	URL       string
	ID        string
	MachineID string
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

// GetMachineID returns the userID of the authenticated user
func GetMachineID() string {
	t, err := getToken()
	if err != nil {
		return ""
	}

	return t.MachineID
}

// GetURL returns the URL of the authenticated user
func GetURL() string {
	t, err := getToken()
	if err != nil {
		return "na"
	}

	return t.URL
}

func saveToken(id, token, url string) error {
	t, err := getToken()
	if err != nil {
		log.Debugf("bad token, re-initializing: %s", err)
		t = &Token{}
	}

	t.ID = id
	t.Token = token
	t.URL = url
	return save(t)
}

// SaveMachineID updates the token file with the machineID value
func SaveMachineID(machineID string) error {
	t, err := getToken()
	if err != nil {
		log.Debugf("bad token, re-initializing: %s", err)
		t = &Token{}
	}

	t.MachineID = machineID
	return save(t)
}

func save(t *Token) error {
	marshalled, err := json.Marshal(t)
	if err != nil {
		log.Infof("failed to marshal token: %s", err)
		return fmt.Errorf("Failed to generate your auth token")
	}

	p := filepath.Join(config.GetHome(), tokenFile)
	log.Debugf("saving token at %s", p)
	if _, err := os.Stat(p); err == nil {
		err = os.Chmod(p, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change token permissions: %s", err)
		}
	}
	if err := ioutil.WriteFile(p, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save authentication token: %s", err)
	}

	return nil
}
