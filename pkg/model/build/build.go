// Copyright 2021 The Okteto Authors
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

package build

import (
	"net/url"

	"github.com/okteto/okteto/pkg/model/environment"
)

// Build represents the build info to generate an image
type Build struct {
	Name       string                  `yaml:"name,omitempty"`
	Context    string                  `yaml:"context,omitempty"`
	Dockerfile string                  `yaml:"dockerfile,omitempty"`
	CacheFrom  []string                `yaml:"cache_from,omitempty"`
	Target     string                  `yaml:"target,omitempty"`
	Args       environment.Environment `yaml:"args,omitempty"`
	Image      string                  `yaml:"image,omitempty"`
}

//SetDefaults sets build default context and dockerfile
func (b *Build) SetDefaults() {
	if b == nil {
		b = &Build{}
	}
	if b.Context == "" {
		b.Context = "."
	}
	if _, err := url.ParseRequestURI(b.Context); err != nil && b.Dockerfile == "" {
		b.Dockerfile = "Dockerfile"
	}
}
