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
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/a8m/envsubst"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type Environment []Var

type VarExpansionErr struct {
	err   error
	value string
}

func (e VarExpansionErr) Error() string {
	return fmt.Sprintf("error expanding environment on '%s': %s", e.value, e.err)
}

// ExpandEnv expands the env vars in the given string (supporting the notation "${var:-$DEFAULT}").
func ExpandEnv(value string) (string, error) {
	result, err := envsubst.String(value)
	if err != nil {
		return "", VarExpansionErr{err, value}
	}
	return result, nil
}

// ExpandEnvIfNotEmpty expands the env vars in the given string (supporting the notation "${var:-$DEFAULT}").
// If the result is an empty string, it returns the original value.
func ExpandEnvIfNotEmpty(value string) (string, error) {
	result, err := ExpandEnv(value)
	if err != nil {
		return "", err
	}
	if result == "" {
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
		value, err = ExpandEnv(value)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// LoadBoolean loads a boolean environment variable and returns it value
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

// LoadBooleanOrDefault loads a boolean environment variable and returns it value
// If the variable is not defined, it returns the default value
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
