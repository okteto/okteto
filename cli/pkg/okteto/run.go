package okteto

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/model"
)

// RunImage runs a docker image
func RunImage(dev *model.Dev) (*Environment, error) {
	c, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting okteto client: %s", err)
	}

	query := fmt.Sprintf(`
	  mutation {
		run(name: "%s", image: "%s") {
		  name,endpoints
		}
	  }`, dev.Name, dev.Image)

	req := graphql.NewRequest(query)

	oktetoToken, err := getToken()
	if err != nil {
		return nil, errors.ErrNotLogged
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()

	var r struct {
		Run Environment
	}

	if err := c.Run(ctx, req, &r); err != nil {
		return nil, fmt.Errorf("error running image: %s", err)
	}

	return &r.Run, nil
}
