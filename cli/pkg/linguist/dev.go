package linguist

import (
	"sort"
	"strings"

	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"
)

type languageDefault struct {
	image   string
	command []string
	path    string
	scripts map[string]string
	forward model.Forward
}

const (
	javascript       = "javascript"
	golang           = "go"
	python           = "python"
	java             = "java"
	ruby             = "ruby"
	helloCommandName = "hello"
	// Unrecognized is the option returned when the linguist couldn't detect a language
	Unrecognized = "other"
)

var (
	languageDefaults map[string]languageDefault
)

func init() {
	languageDefaults = make(map[string]languageDefault)
	languageDefaults[javascript] = languageDefault{
		image: "okteto/node:11",
		path:  "/usr/src/app",
		scripts: map[string]string{
			"test":    "yarn run test",
			"install": "yarn install",
			"start":   "yarn start",
		},
		forward: model.Forward{Local: 3000, Remote: 3000},
	}

	languageDefaults[golang] = languageDefault{
		image: "golang:1",
		path:  "/go/src/app",
		scripts: map[string]string{
			"start": "go run main.go",
		},
		forward: model.Forward{Local: 8080, Remote: 8080},
	}

	languageDefaults[python] = languageDefault{
		image: "python:3",
		path:  "/usr/src/app",
		scripts: map[string]string{
			"install": "pip install -r requirements.txt",
			"start":   "python app.py",
		},
		forward: model.Forward{Local: 8080, Remote: 8080},
	}

	languageDefaults[java] = languageDefault{
		image: "gradle:5.1-jdk11",
		path:  "/home/gradle",
		scripts: map[string]string{
			"boot":  "gradle bootRun",
			"start": "gradle build -continuous --scan",
		},
		forward: model.Forward{Local: 8080, Remote: 8080},
	}

	languageDefaults[ruby] = languageDefault{
		image: "ruby:2",
		path:  "/usr/src/app",
		scripts: map[string]string{
			"migrate": "rails db:migrate",
			"start":   "rails s -e development",
		},
		forward: model.Forward{Local: 3000, Remote: 3000},
	}

	languageDefaults[Unrecognized] = languageDefault{
		image:   "ubuntu:bionic",
		path:    "/usr/src/app",
		forward: model.Forward{Local: 8080, Remote: 8080},
	}
}

// GetSupportedLanguages returns a list of supported languages
func GetSupportedLanguages() []string {
	l := []string{}
	for k := range languageDefaults {
		if k != Unrecognized {
			l = append(l, k)
		}
	}

	sort.Strings(l)
	l = append(l, Unrecognized)

	return l
}

// GetDevConfig returns the default dev for the specified language
func GetDevConfig(language string) *model.Dev {
	vals := languageDefaults[normalizeLanguage(language)]
	dev := &model.Dev{
		Image:   vals.image,
		Command: vals.command,
		WorkDir: &model.Mount{Path: vals.path},
		Scripts: vals.scripts,
		Forward: []model.Forward{vals.forward},
	}
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
		return Unrecognized
	}
}
