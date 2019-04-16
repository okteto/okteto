package okteto

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/model"
)

// Environments top body answer
type Environments struct {
	Environments []Environment
}

// Environment is the information about the dev environment
type Environment struct {
	Name string
}

// DevModeOn activates a dev environment
func DevModeOn(dev *model.Dev, devPath string) error {

	q := fmt.Sprintf(`
	mutation {
		up(name: "%s", image: "%s", workdir: "%s", devPath: "%s") {
			  name
		}
	  }`, dev.Name, dev.Image, dev.WorkDir, devPath)

	if err := query(q, nil); err != nil {
		return fmt.Errorf("failed to activate your dev environment, please try again")
	}

	return nil
}

// GetDevEnvironments returns the name of all the dev environments
func GetDevEnvironments() ([]Environment, error) {
	q := `
	query{
		environments{
		  name,
		}
	}`

	var e Environments
	if err := query(q, &e); err != nil {
		return nil, fmt.Errorf("failed to get your dev environments, please try again")
	}

	return e.Environments, nil
}
