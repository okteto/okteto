// Copyright 2024 The Okteto Authors
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

package vars

import (
	"os"

	"github.com/okteto/okteto/pkg/env"
	"gopkg.in/yaml.v2"
)

type ManifestVars struct {
	Variables Vars `json:"variables,omitempty" yaml:"variables,omitempty"`
}

type Vars []Var

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v Vars) MarshalYAML() (interface{}, error) {
	rawVars := make([]Var, 0)
	for _, val := range v {
		rawVars = append(rawVars, val)

	}
	return rawVars, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *Vars) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawVars map[string]string
	if err := unmarshal(&rawVars); err != nil {
		return err
	}

	for key, val := range rawVars {
		//(*v)[key] = &Var{Name: key, Value: val}
		*v = append(*v, Var{Name: key, Value: val})
	}

	return nil
}

func GetManifestVars(manifestPath string) (Vars, error) {
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var vars ManifestVars
	err = yaml.Unmarshal(b, &vars)
	if err != nil {
		return nil, err
	}

	return vars.Variables, nil
}

func (vars *Vars) Expand() error {
	for i := range *vars {
		expanded, err := env.ExpandEnvIfNotEmpty((*vars)[i].Value)
		if err != nil {
			return err
		}
		(*vars)[i].Value = expanded
	}
	return nil
}

func (vars *Vars) Export(lookupEnv func(key string) (string, bool), setEnv func(key, value string) error, warningLog func(format string, args ...interface{})) error {
	for _, v := range *vars {
		if v.ExistsLocally(lookupEnv) {
			warningLog("The local variable '%s' takes precedence over the manifest's definition, which will be ignored", v.Name)
		}
		if err := setEnv(v.Name, v.Value); err != nil {
			return err
		}
	}
	return nil
}
