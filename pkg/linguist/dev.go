package linguist

import (
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/model"
)

type languageDefault struct {
	image   string
	path    string
	command []string
}

const (
	javascript = "javascript"
	golang     = "go"
	python     = "python"
	java       = "java"
	ruby       = "ruby"

	// Unrecognized is the option returned when the linguist couldn't detect a language
	Unrecognized = "other"
)

var (
	languageDefaults map[string]languageDefault
)

func init() {
	languageDefaults = make(map[string]languageDefault)
	languageDefaults[javascript] = languageDefault{
		image:   "okteto/node:10",
		path:    "/usr/src/app",
		command: []string{"bash"},
	}

	languageDefaults[golang] = languageDefault{
		image:   "okteto/golang:1",
		path:    "/go/src/app",
		command: []string{"bash"},
	}

	languageDefaults[python] = languageDefault{
		image:   "okteto/python:3",
		path:    "/usr/src/app",
		command: []string{"bash"},
	}

	languageDefaults[java] = languageDefault{
		image:   "okteto/gradle:5.1-jdk11",
		path:    "/home/gradle",
		command: []string{"bash"},
	}

	languageDefaults[ruby] = languageDefault{
		image:   "okteto/ruby:2",
		path:    "/usr/src/app",
		command: []string{"bash"},
	}

	languageDefaults[Unrecognized] = languageDefault{
		image:   model.DefaultImage,
		path:    "/usr/src/app",
		command: []string{"bash"},
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
		WorkDir: vals.path,
		Command: vals.command,
	}

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
		return Unrecognized
	}
}
