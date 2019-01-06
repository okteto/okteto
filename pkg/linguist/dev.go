package linguist

import (
	"strings"

	"github.com/okteto/cnd/pkg/model"
	log "github.com/sirupsen/logrus"
)

type languageDefault struct {
	image   string
	command []string
	path    string
}

const (
	node             = "node"
	golang           = "go"
	python           = "python"
	unrecognized     = "unrecognized"
	helloCommandName = "hello"
)

var (
	platformDefaults map[string]languageDefault
	tailCommand      = []string{"tail", "-f", "/dev/null"}
)

func init() {
	platformDefaults = make(map[string]languageDefault)
	platformDefaults[node] = languageDefault{
		image:   "okteto/node:11",
		command: []string{"sh", "-c", "yarn install && yarn start"},
		path:    "/usr/src/app",
	}

	platformDefaults[golang] = languageDefault{
		image:   "golang",
		command: tailCommand,
		path:    "/go/src/app",
	}

	platformDefaults[python] = languageDefault{
		image:   "python",
		command: []string{"python", "app.py"},
		path:    "/usr/src/app",
	}

	platformDefaults[unrecognized] = languageDefault{
		image:   "ubuntu",
		command: tailCommand,
		path:    "/usr/src/app",
	}
}

// GetDevConfig returns the default dev for the specified language
func GetDevConfig(language string) *model.Dev {
	vals := platformDefaults[getPlatform(language)]

	dev := model.NewDev()
	dev.Swap.Deployment.Image = vals.image
	dev.Swap.Deployment.Command = vals.command
	dev.Mount.Source = "."
	dev.Mount.Target = vals.path
	dev.Scripts[helloCommandName] = "echo Your cluster ♥ you"
	return dev
}

func getPlatform(language string) string {
	lower := strings.ToLower(language)
	switch lower {
	case "typescript":
		return node
	case "javascript":
		return node
	case "jsx":
		return node
	case "python":
		return python
	case "go":
		return golang
	default:
		log.Debugf("unrecognized language: %s", lower)
		return unrecognized
	}
}
