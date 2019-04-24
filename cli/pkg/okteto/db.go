package okteto

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/errors"
)

// Database is the database and available endpoint
type Database struct {
	Name     string
	Endpoint string
}

// CreateDatabase creates a cloud database
func CreateDatabase(name string) (*Database, error) {
	c, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting okteto client: %s", err)
	}

	query := fmt.Sprintf(`
	  mutation {
		createDatabase(name: "%s") {
		  name,endpoint
		}
	  }`, name)

	req := graphql.NewRequest(query)

	oktetoToken, err := getToken()
	if err != nil {
		return nil, errors.ErrNotLogged
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()
	var d struct {
		CreateDatabase Database
	}

	if err := c.Run(ctx, req, &d); err != nil {
		return nil, fmt.Errorf("error creating database: %s", err)
	}

	return &d.CreateDatabase, nil
}
