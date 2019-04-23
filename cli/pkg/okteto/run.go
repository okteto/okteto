package okteto

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/model"
)

// RunImage runs a docker image
func RunImage(dev *model.Dev) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting okteto client: %s", err)
	}

	query := fmt.Sprintf(`
	  mutation {
		run(name: "%s", image: "%s") {
		  name
		}
	  }`, dev.Name, dev.Image)

	req := graphql.NewRequest(query)

	oktetoToken, err := getToken()
	if err != nil {
		return errors.ErrNotLogged
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()

	if err := c.Run(ctx, req, nil); err != nil {
		return fmt.Errorf("error running image: %s", err)
	}

	return nil
}
