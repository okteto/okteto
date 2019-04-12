package okteto

import (
	"context"
	"fmt"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/okteto/app/cli/pkg/model"
)

// DevModeOn activates a dev environment
func DevModeOn(dev *model.Dev) error {
	c, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting okteto client: %s", err)
	}

	var services string
	if dev.Services != nil && len(dev.Services) > 0 {
		services = strings.Join(dev.Services, `","`)
		services = fmt.Sprintf(`["%s"]`, services)
	} else {
		services = "[]"
	}

	query := fmt.Sprintf(`
	mutation {
		up(name: "%s", image: "%s", workdir: "%s", services: %s) {
			  name
		}
	  }`, dev.Name, dev.Image, dev.WorkDir, services)

	req := graphql.NewRequest(query)

	oktetoToken, err := getToken()
	if err != nil {
		return fmt.Errorf("please login")
	}

	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", oktetoToken))

	ctx := context.Background()

	if err := c.Run(ctx, req, nil); err != nil {
		return fmt.Errorf("error activating dev environment: %s", err)
	}

	return nil
}
