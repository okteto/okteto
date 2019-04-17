package okteto

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
)

// Environment is the information about the dev environment
type Environment struct {
	Name      string
	Endpoints []string
}

// DevModeOn activates a dev environment
func DevModeOn(dev *model.Dev, devPath string, attach bool) (*Environment, error) {

	q := fmt.Sprintf(`
	mutation {
		up(name: "%s", image: "%s", workdir: "%s", devPath: "%s", attach: %t) {
			  name, endpoints
		}
	  }`, dev.Name, dev.Image, dev.WorkDir, devPath, attach)

	var u struct {
		Up Environment
	}

	if err := query(q, &u); err != nil {
		return nil, fmt.Errorf("failed to activate your dev environment: %s", err)
	}

	return &u.Up, nil
}

// GetDevEnvironments returns the name of all the dev environments
func GetDevEnvironments() ([]Environment, error) {
	q := `
	query{
		environments{
		  name,
		}
	}`

	var e struct {
		Environments []Environment
	}

	if err := query(q, &e); err != nil {
		log.Infof("failed to get your dev environments: %s", err)
		return nil, err
	}

	return e.Environments, nil
}
