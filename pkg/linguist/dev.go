package linguist

import (
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
)

type languageDefault struct {
	image   string
	command []string
	path    string
	scripts map[string]string
}

const (
	javascript       = "javascript"
	golang           = "go"
	python           = "python"
	java             = "java"
	ruby             = "ruby"
	unrecognized     = "unrecognized"
	helloCommandName = "hello"
)

var (
	languageDefaults map[string]languageDefault
	tailCommand      = []string{"tail", "-f", "/dev/null"}
)

func init() {
	languageDefaults = make(map[string]languageDefault)
	languageDefaults[javascript] = languageDefault{
		image:   "okteto/node:11",
		command: []string{"sh", "-c", "yarn install && yarn start"},
		path:    "/usr/src/app",
		scripts: map[string]string{
			"test": "yarn run test",
		},
	}

	languageDefaults[golang] = languageDefault{
		image:   "golang:1",
		command: tailCommand,
		path:    "/go/src/app",
	}

	languageDefaults[python] = languageDefault{
		image:   "python:3",
		command: []string{"sh", "-c", "pip install -r requirements.txt && python app.py"},
		path:    "/usr/src/app",
	}

	languageDefaults[java] = languageDefault{
		image:   "gradle:5.1-jdk11",
		command: []string{"gradle", "build", "-continuous", "--scan"},
		path:    "/home/gradle",
		scripts: map[string]string{
			"boot": "gradle bootRun",
		},
	}

	languageDefaults[ruby] = languageDefault{
		image:   "ruby:2",
		command: tailCommand,
		path:    "/usr/src/app",
		scripts: map[string]string{
			"migrate": "rails db:migrate",
			"server":  "rails s -e development",
		},
	}

	languageDefaults[unrecognized] = languageDefault{
		image:   "ubuntu",
		command: tailCommand,
		path:    "/usr/src/app",
	}
}

// GetDevConfig returns the default dev for the specified language
func GetDevConfig(language string) *model.Dev {
	vals := languageDefaults[normalizeLanguage(language)]
	dev := model.NewDev()
	dev.Swap.Deployment.Image = vals.image
	dev.Swap.Deployment.Command = vals.command
	dev.Mount.Source = "."
	dev.Mount.Target = vals.path
	dev.Scripts = vals.scripts

	if dev.Scripts == nil {
		dev.Scripts = make(map[string]string)
	}

	dev.Scripts[helloCommandName] = "echo Your cluster ♥s you"
	return dev
}

func normalizeLanguage(language string) string {
	lower := strings.ToLower(language)
	switch lower {
	case "typescript":
		return javascript
	case "javascript":
		return javascript
	case "jsx":
		return javascript
	case "python":
		return python
	case "java":
		return java
	case "ruby":
		return ruby
	case "go":
		return golang
	default:
		log.Debugf("unrecognized language: %s", lower)
		return unrecognized
	}
}
