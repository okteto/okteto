package okteto

import (
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/machinebox/graphql"
)

var graphqlClient *graphql.Client
var oktetoToken string

func getClient() (*graphql.Client, error) {
	if graphqlClient == nil {
		oktetoURL := os.Getenv("OKTETO_URL")
		if oktetoURL == "" {
			oktetoURL = "https://cloud.okteto.com"
		}
		u, err := url.Parse(oktetoURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing '%s'", oktetoURL)
		}
		u.Path = path.Join(u.Path, "graphql")
		oktetoURL = u.String()

		oktetoToken = os.Getenv("OKTETO_TOKEN")
		if oktetoToken == "" {
			return nil, fmt.Errorf("'OKTETO_TOKEN' envvar must be defined")
		}
		graphqlClient = graphql.NewClient(oktetoURL)
	}
	return graphqlClient, nil
}
