package okteto

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/model"
)

// RunImage runs a docker image
func RunImage(dev *model.Dev) (*Environment, error) {
	q := ""
	if dev.Space == "" {
		q = fmt.Sprintf(`
		  mutation {
				run(name: "%s", image: "%s") {
		  		name,endpoints
				}
			}`, dev.Name, dev.Image)
	} else {
		q = fmt.Sprintf(`
		  mutation {
				run(name: "%s", image: "%s", space: "%s") {
		  		name,endpoints
				}
			}`, dev.Name, dev.Image, dev.Space)
	}

	var r struct {
		Run Environment
	}

	if err := query(q, &r); err != nil {
		return nil, fmt.Errorf("error running image: %s", err)
	}

	return &r.Run, nil
}
