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

package environment

import (
	"os"
	"sort"
	"strings"
)

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *Environment) UnmarshalYAML(unmarshal func(interface{}) error) error {
	envs := make(Environment, 0)
	result, err := GetKeyValue(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		envs = append(envs, EnvVar{Name: key, Value: value})
	}
	sort.SliceStable(envs, func(i, j int) bool {
		return strings.Compare(envs[i].Name, envs[j].Name) < 0
	})
	*e = envs
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (envFiles *EnvFiles) UnmarshalYAML(unmarshal func(interface{}) error) error {
	result := make(EnvFiles, 0)
	var single string
	err := unmarshal(&single)
	if err != nil {
		var multi []string
		err := unmarshal(&multi)
		if err != nil {
			return err
		}
		result = multi
		*envFiles = result
		return nil
	}

	result = append(result, single)
	*envFiles = result
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *EnvVar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parts := strings.SplitN(raw, "=", 2)
	e.Name = parts[0]
	if len(parts) == 2 {
		e.Value, err = ExpandEnv(parts[1])
		if err != nil {
			return err
		}
		return nil
	}

	e.Name, err = ExpandEnv(parts[0])
	if err != nil {
		return err
	}
	e.Value = os.Getenv(e.Name)
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (e EnvVar) MarshalYAML() (interface{}, error) {
	return e.Name + "=" + e.Value, nil
}

// GetKeyValue returns a map from a list or a map
func GetKeyValue(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []EnvVar
	err := unmarshal(&rawList)
	if err == nil {
		for _, label := range rawList {
			result[label.Name] = label.Value
		}
		return result, nil
	}
	var rawMap map[string]string
	err = unmarshal(&rawMap)
	if err != nil {
		return nil, err
	}
	for key, value := range rawMap {
		value, err = ExpandEnv(value)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}
