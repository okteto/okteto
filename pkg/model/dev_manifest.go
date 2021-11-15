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

package model

import "github.com/okteto/okteto/pkg/log"

//DevManifest represents an okteto manifest
type DevManifest struct {
	Name      string          `json:"name" yaml:"name"`
	Icon      string          `json:"icon,omitempty" yaml:"icon,omitempty"`
	Variables []DevVariables  `json:"variables,omitempty" yaml:"variables,omitempty"`
	Deploy    DeployInfo      `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev       DevManifestDevs `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy   []string        `json:"destroy,omitempty" yaml:"destroy,omitempty"`

	// Build        map[string]BuildInfo         `json:"build,omitempty" yaml:"build,omitempty"`
	// Test         map[string]*TestInfo         `json:"test,omitempty" yaml:"test,omitempty"`
	// Dependencies map[string]*DependenciesInfo `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

type DevManifestDevs map[string]*Dev

func NewDevManifest() *DevManifest {
	return &DevManifest{
		Variables: make([]DevVariables, 0),
		Dev:       make(map[string]*Dev),
		Destroy:   make([]string, 0),
	}
}

func NewDevManifestFromDev(dev *Dev) *DevManifest {
	devManifest := NewDevManifest()
	name, err := ExpandEnv(dev.Name)
	if err != nil {
		log.Infof("could not expand dev name '%s'", dev.Name)
		name = dev.Name
	}
	devManifest.Name = name
	devManifest.Dev[name] = dev
	return devManifest
}

//DevVariables represents a variable
type DevVariables struct {
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Value    string `json:"value,omitempty" yaml:"value,omitempty"`
	Optional bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
}

//DeployInfo represents a deploy section
type DeployInfo struct {
	Commands []string     `json:"commands,omitempty" yaml:"commands,omitempty"`
	Compose  *ComposeInfo `json:"compose,omitempty" yaml:"compose,omitempty"`
	Divert   *DivertInfo  `json:"divert,omitempty" yaml:"divert,omitempty"`
}

func NewDeployInfo() *DeployInfo {
	return &DeployInfo{
		Commands: make([]string, 0),
	}
}

//ComposeInfo represents how to deploy a compose
type ComposeInfo struct {
	Manifest  string         `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	Endpoints []EndpointRule `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}

//DivertInfo represents how to create a divert
type DivertInfo struct {
	From DivertFromInfo `json:"from,omitempty" yaml:"from,omitempty"`
	To   DivertToInfo   `json:"to,omitempty" yaml:"to,omitempty"`
}

//DivertFromInfo represents the service a divert must divert from
type DivertFromInfo struct {
	Namespace  string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Ingress    string `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	Service    string `json:"service,omitempty" yaml:"service,omitempty"`
	Deployment string `json:"deployment,omitempty" yaml:"deployment,omitempty"`
}

//DivertToInfo represents the service a divert must divert to
type DivertToInfo struct {
	Service string `json:"service,omitempty" yaml:"service,omitempty"`
}

// type TestInfo struct {
// 	Image     string               `json:"image,omitempty" yaml:"image,omitempty"`
// 	Variables Environment          `json:"variables,omitempty" yaml:"variables,omitempty"`
// 	Workdir   string               `json:"workdir,omitempty" yaml:"workdir,omitempty"`
// 	Command   Command              `json:"command,omitempty" yaml:"command,omitempty"`
// 	Resources ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
// }

// type DependenciesInfo struct {
// 	Repository string      `json:"repository,omitempty" yaml:"repository,omitempty"`
// 	Manifest   string      `json:"manifest,omitempty" yaml:"manifest,omitempty"`
// 	Branch     string      `json:"branch,omitempty" yaml:"branch,omitempty"`
// 	Variables  Environment `json:"variables,omitempty" yaml:"variables,omitempty"`
// }
