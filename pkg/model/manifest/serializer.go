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
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/model/dev"
)

type manifestRaw struct {
	Namespace string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Context   string   `json:"context,omitempty" yaml:"context,omitempty"`
	Icon      string   `json:"icon,omitempty" yaml:"icon,omitempty"`
	Deploy    *Deploy  `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	Devs      Devs     `json:"dev,omitempty" yaml:"dev,omitempty"`
	Destroy   []string `json:"destroy,omitempty" yaml:"destroy,omitempty"`
	Build     Build    `json:"build,omitempty" yaml:"build,omitempty"`

	DeprecatedDevs []string `yaml:"devs"`
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Manifest) UnmarshalYAML(unmarshal func(interface{}) error) error {
	dev := dev.NewDev()
	err := unmarshal(&dev)
	if err == nil {
		*d = *NewManifestFromDev(dev)
		return nil
	}
	if !isManifestFieldNotFound(err) {
		return err
	}

	manifest := manifestRaw{
		Devs:  Devs{},
		Build: Build{},
	}
	err = unmarshal(&manifest)
	if err != nil {
		return err
	}
	d.Deploy = manifest.Deploy
	d.Destroy = manifest.Destroy
	d.Devs = manifest.Devs
	d.Icon = manifest.Icon
	d.Build = manifest.Build
	d.Namespace = manifest.Namespace
	d.Context = manifest.Context
	return nil
}

func isManifestFieldNotFound(err error) bool {
	manifestFields := []string{"devs", "dev", "name", "icon", "variables", "deploy", "destroy", "build", "namespace", "context"}
	for _, field := range manifestFields {
		if strings.Contains(err.Error(), fmt.Sprintf("field %s not found", field)) {
			return true
		}
	}
	return false
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Deploy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var commands []string
	err := unmarshal(&commands)
	if err == nil {
		d.Commands = commands
		return nil
	}
	return err
}

type devRaw dev.Dev

func (d *devRaw) UnmarshalYAML(unmarshal func(interface{}) error) error {
	devRawPointer := dev.NewDev()
	err := unmarshal(devRawPointer)
	if err != nil {
		return err
	}

	*d = devRaw(*devRawPointer)
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *Devs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type manifestDevsList []string
	devsList := manifestDevsList{}
	err := unmarshal(&devsList)
	if err == nil {
		return nil
	}
	type manifestDevs map[string]devRaw
	devs := manifestDevs{}
	err = unmarshal(&devs)
	if err != nil {
		return err
	}
	result := Devs{}
	for k, v := range devs {
		dev := dev.Dev(v)
		devPointer := &dev
		result[k] = devPointer
	}
	*d = result
	return nil
}
