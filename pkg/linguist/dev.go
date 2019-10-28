package linguist

import (
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

type languageDefault struct {
	image           string
	path            string
	command         []string
	environment     []model.EnvVar
	volumes         []string
	forward         []model.Forward
	securityContext *model.SecurityContext
}

const (
	javascript = "javascript"
	golang     = "go"
	python     = "python"
	gradle     = "gradle"
	maven      = "maven"
	java       = "java"
	ruby       = "ruby"
	csharp     = "csharp"
	vscode     = "vscode"

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
		path:    "/okteto",
		command: []string{"bash"},
		volumes: []string{
			"/go/pkg/",
			"/root/.cache/go-build/",
		},
		securityContext: &model.SecurityContext{
			Capabilities: &model.Capabilities{
				Add: []apiv1.Capability{"SYS_PTRACE"},
			},
		},
		forward: []model.Forward{
			model.Forward{
				Local:  8080,
				Remote: 8080,
			},
			model.Forward{
				Local:  2345,
				Remote: 2345,
			},
		},
	}

	languageDefaults[python] = languageDefault{
		image:   "okteto/python:3",
		path:    "/usr/src/app",
		command: []string{"bash"},
	}

	languageDefaults[gradle] = languageDefault{
		image:   "okteto/gradle:latest",
		path:    "/okteto",
		command: []string{"bash"},
		volumes: []string{"/home/gradle/.gradle"},
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
			{
				Local:  8088,
				Remote: 8088,
			},
		},
	}

	languageDefaults[maven] = languageDefault{
		image:   "okteto/maven:latest",
		path:    "/okteto",
		command: []string{"bash"},
		volumes: []string{"/root/.m2"},
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
			{
				Local:  8088,
				Remote: 8088,
			},
		},
	}

	languageDefaults[ruby] = languageDefault{
		image:   "okteto/ruby:2",
		path:    "/usr/src/app",
		command: []string{"bash"},
	}

	languageDefaults[csharp] = languageDefault{
		image:   "mcr.microsoft.com/dotnet/core/sdk",
		command: []string{"bash"},
		environment: []model.EnvVar{
			{
				Name:  "ASPNETCORE_ENVIRONMENT",
				Value: "Development",
			},
		},
		forward: []model.Forward{
			{
				Local:  5000,
				Remote: 5000,
			},
		},
	}

	languageDefaults[vscode] = languageDefault{
		image:   model.DefaultImage,
		command: []string{"bash"},
		volumes: []string{
			"/root/.vscode-server",
		},
		securityContext: &model.SecurityContext{
			Capabilities: &model.Capabilities{
				Add: []apiv1.Capability{
					apiv1.Capability("SYS_PTRACE"),
				},
			},
		},
	}

	languageDefaults[Unrecognized] = languageDefault{
		image:   model.DefaultImage,
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
	n := normalizeLanguage(language)
	vals := languageDefaults[n]
	dev := &model.Dev{
		Image:           vals.image,
		WorkDir:         vals.path,
		Command:         vals.command,
		Environment:     vals.environment,
		Volumes:         vals.volumes,
		Forward:         vals.forward,
		SecurityContext: vals.securityContext,
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
	case "node":
		return javascript
	case "python":
		return python
	case "java":
		return gradle
	case "gradle":
		return gradle
	case "maven":
		return maven
	case "ruby":
		return ruby
	case "go":
		return golang
	case "c#":
		return csharp
	case "csharp":
		return csharp
	default:
		return Unrecognized
	}
}
