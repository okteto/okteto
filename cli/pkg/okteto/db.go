package okteto

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/errors"
)

// CreateDatabase creates a cloud database
func CreateDatabase(name string) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting okteto client: %s", err)
	}

	query := fmt.Sprintf(`
	  mutation {
		createDatabase(name: "%s") {
		  name
		}
	  }`, name)

	req := graphql.NewRequest(query)

	oktetoToken, err := getToken()
	if err != nil {
		return errors.ErrNotLogged
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()

	if err := c.Run(ctx, req, nil); err != nil {
		return fmt.Errorf("error creating database: %s", err)
	}

	return nil
}
