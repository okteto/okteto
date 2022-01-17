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

package manifest

import (
	"os"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/build"
	"github.com/okteto/okteto/pkg/model/dev"
	"github.com/okteto/okteto/pkg/model/environment"
)

//Type represents the type of manifest
type Type string

var (
	//StackType represents a stack manifest type
	StackType Type = "stack"
	//OktetoType represents a okteto manifest type
	OktetoType Type = "okteto"
	//KubernetesType represents a k8s manifest type
	KubernetesType Type = "kubernetes"
	//ChartType represents a k8s manifest type
	ChartType Type = "chart"
)

//Manifest represents an okteto manifest
type Manifest struct {
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context   string   `json:"context,omitempty" yaml:"context,omitempty"`
	Icon      string   `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy    *Deploy  `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Devs      Devs     `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy   []string `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build     Build    `json:"build,omitempty" yaml:"build,omitempty"`

	Type     Type   `json:"-" yaml:"-"`
	Filename string `json:"-" yaml:"-"`
}

//Devs defines all the dev section
type Devs map[string]*dev.Dev

//Build defines all the build section
type Build map[string]*build.Build

//NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Devs:  Devs{},
		Build: Build{},
	}
}

//NewManifestFromDev creates a manifest from a dev
func NewManifestFromDev(dev *dev.Dev) *Manifest {
	manifest := NewManifest()
	name, err := environment.ExpandEnv(dev.Name)
	if err != nil {
		log.Infof("could not expand dev name '%s'", dev.Name)
		name = dev.Name
	}
	manifest.Devs[name] = dev
	return manifest
}

//SetName sets manifest name
func (m *Manifest) SetName(name string) {
	m.Name = name
	if err := os.Setenv("OKTETO_APP_NAME", name); err != nil {
		log.Infof("invalid app name: %s", err)
	}
	if m.Type == ChartType {
		environment.ExpandEnv(m.Deploy.Commands[0])
	}
}
