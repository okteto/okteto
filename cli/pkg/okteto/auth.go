package okteto

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/config"
)

// Auth authenticates in okteto with a github OAuth code
func Auth(ctx context.Context, code string) (string, error) {
	client, err := getClient()
	if err != nil {
		return "", err
	}

	q := fmt.Sprintf(`
				mutation {
					auth(code: "%s") {
					  id,token
					}
				  }`, code)

	req := graphql.NewRequest(q)

	type User struct {
		Auth struct {
			ID    string
			Token string
		}
	}

	var user User
	if err := client.Run(ctx, req, &user); err != nil {
		return "", fmt.Errorf("unauthorized request: %s", err)
	}

	if len(user.Auth.ID) == 0 || len(user.Auth.Token) == 0 {
		return "", fmt.Errorf("empty response")
	}

	if err := saveToken(user.Auth.Token); err != nil {
		return "", err
	}

	return user.Auth.ID, nil
}

func getTokenPath() string {
	return filepath.Join(config.GetHome(), ".token")
}

func saveToken(token string) error {
	h := config.GetHome()
	if err := ioutil.WriteFile(filepath.Join(h, ".token"), []byte(token), 400); err != nil {
		return fmt.Errorf("couldn't save authentication token: %s", err)
	}

	return nil
}
