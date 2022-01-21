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

import (
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

//Manifest represents an okteto manifest
type Manifest struct {
	Namespace string        `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context   string        `json:"context,omitempty" yaml:"context,omitempty"`
	Icon      string        `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy    *DeployInfo   `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Dev       ManifestDevs  `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy   []string      `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build     ManifestBuild `json:"build,omitempty" yaml:"build,omitempty"`

	Type     string `json:"-" yaml:"-"`
	Filename string `json:"-" yaml:"-"`
}

type ManifestDevs map[string]*Dev
type ManifestBuild map[string]*BuildInfo

func NewManifest() *Manifest {
	return &Manifest{
		Dev: make(map[string]*Dev),
	}
}

func NewManifestFromDev(dev *Dev) *Manifest {
	manifest := NewManifest()
	name, err := ExpandEnv(dev.Name)
	if err != nil {
		oktetoLog.Infof("could not expand dev name '%s'", dev.Name)
		name = dev.Name
	}
	manifest.Dev[name] = dev
	return manifest
}

//DeployInfo represents a deploy section
type DeployInfo struct {
	Commands []string `json:"commands,omitempty" yaml:"commands,omitempty"`
}

func NewDeployInfo() *DeployInfo {
	return &DeployInfo{
		Commands: make([]string, 0),
	}
}
