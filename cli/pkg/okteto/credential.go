package okteto

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
)

// Credentials top body answer
type Credentials struct {
	Credentials Credential
}

// Credential field answer
type Credential struct {
	Config string
}

// GetK8sB64Config returns the space config credentials
func GetK8sB64Config() (string, error) {
	c, err := getClient()
	if err != nil {
		return "", fmt.Errorf("error getting okteto client: %s", err)
	}

	req := graphql.NewRequest(`
		query{
			credentials{
				config
			},
		}`)

	oktetoToken, err := getToken()
	if err != nil {
		return "", fmt.Errorf("authorization token not found. please run 'okteto login' and try again")
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()

	var cred Credentials
	if err := c.Run(ctx, req, &cred); err != nil {
		return "", fmt.Errorf("couldn't get your credentials, please run 'okteto login' and try again")
	}

	return cred.Credentials.Config, nil
}
