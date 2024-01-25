// Copyright 2023 The Okteto Authors
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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/model/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	apiv1 "k8s.io/api/core/v1"
)

type languageDefault struct {
	securityContext *model.SecurityContext
	image           string
	path            string
	command         []string
	environment     []env.Var
	volumes         []model.Volume
	forward         []forward.Forward
	reverse         []model.Reverse
	remote          int
}

const (
	Javascript = "javascript"
	golang     = "go"
	Python     = "python"
	Gradle     = "gradle"
	Maven      = "maven"
	Java       = "java"
	Ruby       = "ruby"
	Csharp     = "csharp"
	Php        = "php"
	Rust       = "rust"

	// Unrecognized is the option returned when the linguist couldn't detect a language
	Unrecognized = "other"
)

var (
	languageDefaults map[string]languageDefault
	forwardDefaults  map[string][]forward.Forward
)

func init() {
	languageDefaults = make(map[string]languageDefault)
	forwardDefaults = make(map[string][]forward.Forward)
	languageDefaults[Javascript] = languageDefault{
		image:   "okteto/node:14",
		path:    "/usr/src/app",
		command: []string{"bash"}, forward: []forward.Forward{
			{
				Local:  9229,
				Remote: 9229,
			},
		},
	}
	forwardDefaults[Javascript] = []forward.Forward{
		{
			Local:  3000,
			Remote: 3000,
		},
	}

	languageDefaults[golang] = languageDefault{
		image:   "okteto/golang:1",
		path:    "/usr/src/app",
		command: []string{"bash"},
		securityContext: &model.SecurityContext{
			Capabilities: &model.Capabilities{
				Add: []apiv1.Capability{"SYS_PTRACE"},
			},
		},
		forward: []forward.Forward{
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
	forwardDefaults[golang] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Rust] = languageDefault{
		image:   "okteto/rust:1",
		path:    "/usr/src/app",
		command: []string{"bash"},
		volumes: []model.Volume{
			{
				RemotePath: "/usr/local/cargo/registry",
			},
			{
				RemotePath: "/home/root/app/target",
			},
		},
	}
	forwardDefaults[Rust] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Python] = languageDefault{
		image:   "okteto/python:3",
		path:    "/usr/src/app",
		command: []string{"bash"},
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
	forwardDefaults[Python] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Gradle] = languageDefault{
		image:   "okteto/gradle:6.5",
		path:    "/usr/src/app",
		command: []string{"bash"},
		forward: []forward.Forward{
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
	forwardDefaults[Gradle] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Maven] = languageDefault{
		image:   "okteto/maven:3",
		path:    "/usr/src/app",
		command: []string{"bash"},
		forward: []forward.Forward{
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
	forwardDefaults[Maven] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Ruby] = languageDefault{
		image:   "okteto/ruby:2",
		path:    "/usr/src/app",
		command: []string{"bash"},
		forward: []forward.Forward{
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
	forwardDefaults[Ruby] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Csharp] = languageDefault{
		image:   "okteto/dotnetcore:3",
		path:    "/usr/src/app",
		command: []string{"bash"},
		environment: []env.Var{
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
		forward: []forward.Forward{},
		remote:  22000,
	}
	forwardDefaults[Csharp] = []forward.Forward{
		{
			Local:  5000,
			Remote: 5000,
		},
	}

	languageDefaults[Php] = languageDefault{
		image:   "okteto/php:7",
		path:    "/usr/src/app",
		command: []string{"bash"},
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
	forwardDefaults[Php] = []forward.Forward{
		{
			Local:  8080,
			Remote: 8080,
		},
	}

	languageDefaults[Unrecognized] = languageDefault{
		image:   model.DefaultImage,
		path:    "/usr/src/app",
		command: []string{"bash"},
		forward: []forward.Forward{},
	}
	forwardDefaults[Unrecognized] = []forward.Forward{
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
func GetDevDefaults(language, workdir string, imageConfig registry.ImageMetadata) (*model.Dev, error) {
	language = NormalizeLanguage(language)
	vals := languageDefaults[language]

	if imageConfig.Workdir == "" || imageConfig.Workdir == "/" {
		imageConfig.Workdir = vals.path
	}

	if filepath.IsAbs(workdir) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workdir, err = filepath.Rel(wd, workdir)
		if err != nil {
			return nil, err
		}
	}

	dev := &model.Dev{
		Image: &build.Info{
			Name: vals.image,
		},
		Command: model.Command{
			Values: vals.command,
		},
		Environment: vals.environment,
		Volumes:     vals.volumes,
		Sync: model.Sync{
			RescanInterval: model.DefaultSyncthingRescanInterval,
			Folders: []model.SyncFolder{
				{
					LocalPath:  workdir,
					RemotePath: imageConfig.Workdir,
				},
			},
		},
		Forward:         vals.forward,
		Reverse:         vals.reverse,
		RemotePort:      vals.remote,
		SecurityContext: vals.securityContext,
	}

	name, err := utils.GetValidNameFromFolder(workdir)
	if err != nil {
		return nil, err
	}
	dev.Name = name
	dev.Context = okteto.GetContext().Name
	dev.Namespace = okteto.GetContext().Namespace
	return dev, nil
}

// SetForwardDefaults set port forward default values for the specified language
func SetForwardDefaults(dev *model.Dev, language string) {
	language = NormalizeLanguage(language)
	vals := forwardDefaults[language]
	if dev.Forward == nil {
		dev.Forward = []forward.Forward{}
	}
	dev.Forward = append(dev.Forward, vals...)
}

func NormalizeLanguage(language string) string {
	lower := strings.ToLower(language)
	switch lower {
	case "typescript", "javascript", "jsx", "node", "tsx":
		return Javascript
	case "python":
		return Python
	case "java":
		return Gradle
	case "gradle":
		return Gradle
	case "maven":
		return Maven
	case "ruby":
		return Ruby
	case "go", "golang":
		return golang
	case "c#":
		return Csharp
	case "csharp":
		return Csharp
	case "php":
		return Php
	case "rust":
		return Rust
	default:
		return Unrecognized
	}
}
