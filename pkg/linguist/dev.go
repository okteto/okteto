// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	volumes         []model.Volume
	forward         []model.Forward
	reverse         []model.Reverse
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
	php        = "php"

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
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
			{
				Local:  5005,
				Remote: 5005,
			},
		},
	}

	languageDefaults[maven] = languageDefault{
		image:   "okteto/maven:latest",
		path:    "/okteto",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
			{
				Local:  5005,
				Remote: 5005,
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

	languageDefaults[php] = languageDefault{
		image:   "okteto/php:7",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
		},
		reverse: []model.Reverse{
			{
				Local:  9000,
				Remote: 9000,
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
		Image:   vals.image,
		WorkDir: vals.path,
		Command: model.Command{
			Values: vals.command,
		},
		Environment:     vals.environment,
		Volumes:         vals.volumes,
		Forward:         vals.forward,
		Reverse:         vals.reverse,
		SecurityContext: vals.securityContext,
	}

	return dev
}

func normalizeLanguage(language string) string {
	lower := strings.ToLower(language)
	switch lower {
	case "typescript", "javascript", "jsx", "node", "tsx":
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
	case "go", "golang":
		return golang
	case "c#":
		return csharp
	case "csharp":
		return csharp
	case "php":
		return php
	default:
		return Unrecognized
	}
}
