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
	remote          int
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
	languageDefaults               map[string]languageDefault
	languageDefaultsWithDeployment map[string]languageDefault
)

func init() {
	languageDefaults = make(map[string]languageDefault)
	languageDefaultsWithDeployment = make(map[string]languageDefault)
	languageDefaults[javascript] = languageDefault{
		image:   "okteto/node:10",
		path:    "/usr/src/app",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  3000,
				Remote: 3000,
			},
			{
				Local:  9229,
				Remote: 9229,
			},
		},
	}
	languageDefaultsWithDeployment[javascript] = languageDefault{
		forward: []model.Forward{
			{
				Local:  9229,
				Remote: 9229,
			},
		},
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
			{
				Local:  8080,
				Remote: 8080,
			},
			{
				Local:  2345,
				Remote: 2345,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/go/pkg/",
			},
			{
				MountPath: "/root/.cache/go-build/",
			},
		},
	}
	languageDefaultsWithDeployment[golang] = languageDefault{
		image:   "okteto/golang:1",
		path:    "/okteto",
		command: []string{"bash"},
		securityContext: &model.SecurityContext{
			Capabilities: &model.Capabilities{
				Add: []apiv1.Capability{"SYS_PTRACE"},
			},
		},
		forward: []model.Forward{
			{
				Local:  2345,
				Remote: 2345,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/go/pkg/",
			},
			{
				MountPath: "/root/.cache/go-build/",
			},
		},
	}

	languageDefaults[python] = languageDefault{
		image:   "okteto/python:3",
		path:    "/usr/src/app",
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
		volumes: []model.Volume{
			{
				MountPath: "/root/.cache/pip",
			},
		},
	}
	languageDefaultsWithDeployment[python] = languageDefault{
		forward: []model.Forward{},
		reverse: []model.Reverse{
			{
				Local:  9000,
				Remote: 9000,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/root/.cache/pip",
			},
		},
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
		volumes: []model.Volume{
			{
				MountPath: "/home/gradle/.gradle",
			},
		},
	}
	languageDefaultsWithDeployment[gradle] = languageDefault{
		image:   "okteto/gradle:latest",
		path:    "/okteto",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  5005,
				Remote: 5005,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/home/gradle/.gradle",
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
		volumes: []model.Volume{
			{
				MountPath: "/root/.m2",
			},
		},
	}
	languageDefaultsWithDeployment[maven] = languageDefault{
		image:   "okteto/maven:latest",
		path:    "/okteto",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  5005,
				Remote: 5005,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/root/.m2",
			},
		},
	}

	languageDefaults[ruby] = languageDefault{
		image:   "okteto/ruby:2",
		path:    "/usr/src/app",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
			{
				Local:  1234,
				Remote: 1234,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/usr/local/bundle/cache",
			},
		},
	}
	languageDefaultsWithDeployment[ruby] = languageDefault{
		forward: []model.Forward{
			{
				Local:  1234,
				Remote: 1234,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/usr/local/bundle/cache",
			},
		},
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
		remote: 22000,
		forward: []model.Forward{
			{
				Local:  5000,
				Remote: 5000,
			},
		},
	}
	languageDefaultsWithDeployment[csharp] = languageDefault{
		image:   "mcr.microsoft.com/dotnet/core/sdk",
		command: []string{"bash"},
		environment: []model.EnvVar{
			{
				Name:  "ASPNETCORE_ENVIRONMENT",
				Value: "Development",
			},
		},
		remote:  2222,
		forward: []model.Forward{},
	}

	languageDefaults[php] = languageDefault{
		image:   "okteto/php:7",
		path:    "/usr/src/app",
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
		volumes: []model.Volume{
			{
				MountPath: "/root/.composer/cache",
			},
		},
	}
	languageDefaultsWithDeployment[php] = languageDefault{
		forward: []model.Forward{},
		reverse: []model.Reverse{
			{
				Local:  9000,
				Remote: 9000,
			},
		},
		volumes: []model.Volume{
			{
				MountPath: "/root/.composer/cache",
			},
		},
	}

	languageDefaults[Unrecognized] = languageDefault{
		image:   model.DefaultImage,
		path:    "/okteto",
		command: []string{"bash"},
		forward: []model.Forward{
			{
				Local:  8080,
				Remote: 8080,
			},
		},
	}
	languageDefaultsWithDeployment[Unrecognized] = languageDefault{
		image:   model.DefaultImage,
		path:    "/okteto",
		command: []string{"bash"},
		forward: []model.Forward{},
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

// GetDevDefaults gets default values for the specified language
func GetDevDefaults(language, workdir string, iAskingForDeployment bool) (*model.Dev, error) {
	language = normalizeLanguage(language)
	vals := languageDefaults[language]
	if iAskingForDeployment {
		vals = languageDefaultsWithDeployment[language]
	}

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
		RemotePort:      vals.remote,
		SecurityContext: vals.securityContext,
	}
	if len(dev.Volumes) > 0 {
		dev.PersistentVolumeInfo = &model.PersistentVolumeInfo{
			Enabled: true,
		}
	}

	name, err := model.GetValidNameFromFolder(workdir)
	if err != nil {
		return nil, err
	}
	dev.Name = name
	return dev, nil
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
