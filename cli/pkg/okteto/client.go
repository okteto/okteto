package okteto

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/log"
)

var graphqlClient *graphql.Client

func getClient() (*graphql.Client, error) {
	if graphqlClient == nil {
		oktetoURL := GetURL()
		u, err := url.Parse(oktetoURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing '%s'", oktetoURL)
		}
		u.Path = path.Join(u.Path, "graphql")
		oktetoURL = u.String()

		graphqlClient = graphql.NewClient(oktetoURL)
	}
	return graphqlClient, nil
}

func getToken() (string, error) {
	if t := os.Getenv("OKTETO_TOKEN"); len(t) > 0 {
		log.Info("using token from environment")
		return t, nil
	}

	p := getTokenPath()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func query(query string, result interface{}) error {
	oktetoToken, err := getToken()
	if err != nil {
		log.Infof("couldn't get token for up: %s", err)
		return errors.ErrNotLogged
	}

	req := graphql.NewRequest(query)
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))
	ctx := context.Background()

	c, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting okteto client: %s", err)
	}

	if err := c.Run(ctx, req, result); err != nil {
		return err
	}

	return nil
}

// GetURL returns the okteto URL
func GetURL() string {
	oktetoURL := os.Getenv("OKTETO_URL")
	if oktetoURL == "" {
		oktetoURL = "https://cloud.okteto.com"
	}
	return oktetoURL
}
