package okteto

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/model"
)

// RunImage runs a docker image
func RunImage(dev *model.Dev) (*Environment, error) {
	q := fmt.Sprintf(`
	  mutation {
		run(name: "%s", image: "%s") {
		  name,endpoints
		}
	  }`, dev.Name, dev.Image)

	var r struct {
		Run Environment
	}

	if err := query(q, &r); err != nil {
		return nil, fmt.Errorf("error running image: %s", err)
	}

	return &r.Run, nil
}
