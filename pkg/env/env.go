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

package env

import (
	"fmt"
	"github.com/a8m/envsubst"
	"sort"
	"strings"
)

type Environment []Var

// ExpandEnv expands the environments supporting the notation "${var:-$DEFAULT}"
func ExpandEnv(value string, expandIfEmpty bool) (string, error) {
	result, err := envsubst.String(value)
	if err != nil {
		return "", fmt.Errorf("error expanding environment on '%s': %s", value, err.Error())
	}
	if result == "" && !expandIfEmpty {
		return value, nil
	}
	return result, nil
}

func (e *Environment) UnmarshalYAML(unmarshal func(interface{}) error) error {
	envs := make(Environment, 0)
	result, err := getKeyValue(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		envs = append(envs, Var{Name: key, Value: value})
	}
	sort.SliceStable(envs, func(i, j int) bool {
		return strings.Compare(envs[i].Name, envs[j].Name) < 0
	})
	*e = envs
	return nil
}

func getKeyValue(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []Var
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
		value, err = ExpandEnv(value, true)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}
