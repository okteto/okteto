package okteto

import (
	"os"
	"net/url"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"encoding/json"
	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/log"
)

const(
	tokenFile = ".token.json"

	// CloudURL is the default URL of okteto
	CloudURL = "https://cloud.okteto.com"
)

// Token contains the auth token and the URL it belongs to
type Token struct {
	Token string
	URL string
}

// Auth authenticates in okteto with a github OAuth code
func Auth(ctx context.Context, code, url string) (string, error) {
	client, err := getClient(url)
	if err != nil {
		return "", err
	}

	q := fmt.Sprintf(`
				mutation {
					auth(code: "%s") {
					  githubID,token
					}
				  }`, code)

	req := graphql.NewRequest(q)

	type User struct {
		Auth struct {
			GithubID string
			Token    string
		}
	}

	var user User
	if err := client.Run(ctx, req, &user); err != nil {
		return "", fmt.Errorf("unauthorized request: %s", err)
	}

	if len(user.Auth.GithubID) == 0 || len(user.Auth.Token) == 0 {
		return "", fmt.Errorf("empty response")
	}

	if err := saveToken(user.Auth.Token, url); err != nil {
		return "", err
	}

	return user.Auth.GithubID, nil
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
	if len(os.Getenv("OKTETO_TOKEN")) > 0 {
		return getTokenFromEnv()
	}

	p := filepath.Join(config.GetHome(), tokenFile)

	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	t := &Token{}
	err = json.Unmarshal(b, t);
	return t, err
}

func saveToken(token, url string) error {
	t := Token{Token: token, URL: url}
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
