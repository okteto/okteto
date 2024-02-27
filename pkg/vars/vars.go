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
	"fmt"
	"os"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

type ManifestVars struct {
	Variables Vars `json:"variables,omitempty" yaml:"variables,omitempty"`
}

type Vars []Var

func GetManifestVars(manifestPath string, fs afero.Fs) (Vars, error) {
	b, err := afero.ReadFile(fs, manifestPath)
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

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (vars Vars) MarshalYAML() (interface{}, error) {
	vMap := make(map[string]string)
	for _, v := range vars {
		vMap[v.Name] = v.Value
	}
	return vMap, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (vars *Vars) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawVars map[string]string
	if err := unmarshal(&rawVars); err != nil {
		return err
	}

	for key, val := range rawVars {
		//(*v)[key] = &Var{Name: key, Value: val}
		*vars = append(*vars, Var{Name: key, Value: val})
	}

	return nil
}

func (vars *Vars) Expand(expandEnv func(string) (string, error)) error {
	for i := range *vars {
		expanded, err := expandEnv((*vars)[i].Value)
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
			warningLog("Local variable '%s' takes precedence over the manifest's definition, which will be ignored", v.Name)
			fmt.Printf("--->DEBUG ... %s = %s\n", v.Name, os.Getenv(v.Name))
			continue
		}
		if err := setEnv(v.Name, v.Value); err != nil {
			return err
		}
	}
	return nil
}

func (vars *Vars) Mask(maskFn func(string)) {
	for _, v := range *vars {
		maskFn(v.Value)
	}
}
