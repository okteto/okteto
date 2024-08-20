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
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/vars"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Environment []vars.Var

func (e *Environment) UnmarshalYAML(unmarshal func(interface{}) error) error {
	envs := make(Environment, 0)
	result, err := getKeyValue(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		envs = append(envs, vars.Var{Name: key, Value: value})
	}
	sort.SliceStable(envs, func(i, j int) bool {
		return strings.Compare(envs[i].Name, envs[j].Name) < 0
	})
	*e = envs
	return nil
}

func getKeyValue(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []vars.Var
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
		value, err = vars.GlobalVarManager.Expand(value)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

func LoadBoolean(k string) bool {
	v := os.Getenv(k)
	if v == "" {
		v = "false"
	}

	h, err := strconv.ParseBool(v)
	if err != nil {
		oktetoLog.Yellow("'%s' is not a valid value for environment variable %s", v, k)
	}

	return h
}

func LoadBooleanOrDefault(k string, d bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return d
	}

	h, err := strconv.ParseBool(v)
	if err != nil {
		oktetoLog.Yellow("'%s' is not a valid value for environment variable %s", v, k)
	}

	return h
}
