package okteto

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/model"
)

// DevModeOn activates a dev environment
func DevModeOn(dev *model.Dev, devPath string) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting okteto client: %s", err)
	}

	query := fmt.Sprintf(`
	mutation {
		up(name: "%s", image: "%s", workdir: "%s", devPath: "%s") {
			  name
		}
	  }`, dev.Name, dev.Image, dev.WorkDir, devPath)

	req := graphql.NewRequest(query)

	oktetoToken, err := getToken()
	if err != nil {
		return errNoLogin
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()

	if err := c.Run(ctx, req, nil); err != nil {
		return fmt.Errorf("failed to activate your dev environment, please try again")
	}

	return nil
}
