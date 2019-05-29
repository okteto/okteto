package okteto

import (
	"fmt"
	"strings"

	"github.com/okteto/app/cli/pkg/model"
)

// Environment is the information about the dev environment
type Environment struct {
	Name      string
	Endpoints []string
}

// DevModeOn activates a dev environment
func DevModeOn(dev *model.Dev, devPath string, attach bool) (*Environment, error) {
	volumes := "[]"
	if len(dev.Volumes) > 0 {
		volumes = strings.Join(dev.Volumes, `", "`)
		volumes = fmt.Sprintf(`["%s"]`, volumes)
	}

	q := ""
	if dev.Space == "" {
		q = fmt.Sprintf(`
		mutation {
			up(name: "%s", image: "%s", workdir: "%s", devPath: "%s", volumes: %s, attach: %t) {
				name, endpoints
			}
		}`, dev.Name, dev.Image, dev.WorkDir, devPath, volumes, attach)
	} else {
		q = fmt.Sprintf(`
		mutation {
			up(name: "%s", image: "%s", workdir: "%s", devPath: "%s", volumes: %s, attach: %t, space: "%s") {
				name, endpoints
			}
		}`, dev.Name, dev.Image, dev.WorkDir, devPath, volumes, attach, dev.Space)

	}

	var u struct {
		Up Environment
	}

	if err := query(q, &u); err != nil {
		msg := strings.TrimLeft(err.Error(), "graphql: ")
		return nil, fmt.Errorf("failed to activate your dev environment: %s", msg)
	}

	return &u.Up, nil
}
