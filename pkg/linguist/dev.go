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
	rust       = "rust"

	// Unrecognized is the option returned when the linguist couldn't detect a language
	Unrecognized = "other"
)

var (
	languageDefaults map[string]languageDefault
	forwardDefaults  map[string][]model.Forward
)

func init() {
	languageDefaults = make(map[string]languageDefault)
	forwardDefaults = make(map[string][]model.Forward)
	languageDefaults[javascript] = languageDefault{
		image: "okteto/node:12",
		forward: []model.Forward{
			{
				Local:  9229,
				Remote: 9229,
			},
		},
	}
	forwardDefaults[javascript] = []model.Forward{
		{
			Local:  3000,
			Remote: 3000,
		},
	}

	languageDefaults[golang] = languageDefault{
		image: "okteto/golang:1",
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
				RemotePath: "/go/pkg/",
			},
			{
				RemotePath: "/root/.cache/go-build/",
			},
		},
	}
	forwardDefaults[golang] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[rust] = languageDefault{
		image: "okteto/rust:1",
		volumes: []model.Volume{
			{
				RemotePath: "/usr/local/cargo/registry",
			},
			{
				RemotePath: "/home/root/app/target",
			},
		},
	}
	forwardDefaults[golang] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[python] = languageDefault{
		image: "okteto/python:3",
		reverse: []model.Reverse{
			{
				Local:  9000,
				Remote: 9000,
			},
		},
		volumes: []model.Volume{
			{
				RemotePath: "/root/.cache/pip",
			},
		},
	}
	forwardDefaults[python] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[gradle] = languageDefault{
		image: "okteto/gradle:6.5",
		forward: []model.Forward{
			{
				Local:  5005,
				Remote: 5005,
			},
		},
		volumes: []model.Volume{
			{
				RemotePath: "/home/gradle/.gradle",
			},
		},
	}
	forwardDefaults[gradle] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[maven] = languageDefault{
		image: "okteto/maven:3",
		forward: []model.Forward{
			{
				Local:  5005,
				Remote: 5005,
			},
		},
		volumes: []model.Volume{
			{
				RemotePath: "/root/.m2",
			},
		},
	}
	forwardDefaults[maven] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[ruby] = languageDefault{
		image: "okteto/ruby:2",
		forward: []model.Forward{
			{
				Local:  1234,
				Remote: 1234,
			},
		},
		volumes: []model.Volume{
			{
				RemotePath: "/usr/local/bundle/cache",
			},
		},
	}
	forwardDefaults[ruby] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[csharp] = languageDefault{
		image: "okteto/dotnetcore:3",
		environment: []model.EnvVar{
			{
				Name:  "ASPNETCORE_ENVIRONMENT",
				Value: "Development",
			},
			{
				Name:  "VSTEST_HOST_DEBUG",
				Value: "0",
			},
			{
				Name:  "VSTEST_RUNNER_DEBUG",
				Value: "0",
			},
		},
		forward: []model.Forward{},
		remote:  22000,
	}
	forwardDefaults[csharp] = []model.Forward{
		{
			Local:  5000,
			Remote: 5000,
		},
	}

	languageDefaults[php] = languageDefault{
		image: "okteto/php:7",
		reverse: []model.Reverse{
			{
				Local:  9000,
				Remote: 9000,
			},
		},
		volumes: []model.Volume{
			{
				RemotePath: "/root/.composer/cache",
			},
		},
	}
	forwardDefaults[php] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Unrecognized] = languageDefault{
		image:   model.DefaultImage,
		forward: []model.Forward{},
	}
	forwardDefaults[Unrecognized] = []model.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
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

	dev := &model.Dev{
		Image: &model.BuildInfo{
			BuildInfoRaw: model.BuildInfoRaw{
				Name: vals.image,
			},
		},
		Command: model.Command{
			Values: []string{"bash"},
		},
		Environment: vals.environment,
		Volumes:     vals.volumes,
		Syncs: []model.Sync{
			{
				LocalPath:  ".",
				RemotePath: "/usr/src/app",
			},
		},
		Forward:         vals.forward,
		Reverse:         vals.reverse,
		RemotePort:      vals.remote,
		SecurityContext: vals.securityContext,
	}

	name, err := model.GetValidNameFromFolder(workdir)
	if err != nil {
		return nil, err
	}
	dev.Name = name
	return dev, nil
}

// SetForwardDefaults set port forward default values for the specified language
func SetForwardDefaults(dev *model.Dev, language string) {
	language = normalizeLanguage(language)
	vals := forwardDefaults[language]
	if dev.Forward == nil {
		dev.Forward = []model.Forward{}
	}
	dev.Forward = append(dev.Forward, vals...)
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
	case "rust":
		return rust
	default:
		return Unrecognized
	}
}
