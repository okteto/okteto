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

package build

import (
	"fmt"
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/env"
)

// Arg is an argument used on the build step.
type Arg struct {
	Name  string
	Value string
}

// Args is a list of arguments used on the build step.
type Args []Arg

func (a *Arg) String() string {
	value, err := env.ExpandEnv(a.Value)
	if err != nil {
		return fmt.Sprintf("%s=%s", a.Name, a.Value)
	}
	return fmt.Sprintf("%s=%s", a.Name, value)
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (a *Arg) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}
	maxBuildArgsParts := 2

	parts := strings.SplitN(raw, "=", maxBuildArgsParts)
	a.Name = parts[0]
	if len(parts) == maxBuildArgsParts {
		a.Value = parts[1]
		return nil
	}

	a.Name, err = env.ExpandEnv(parts[0])
	if err != nil {
		return err
	}
	a.Value = parts[0]
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (a *Args) UnmarshalYAML(unmarshal func(interface{}) error) error {
	buildArgs := make(Args, 0)
	result, err := getArgs(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		buildArgs = append(buildArgs, Arg{Name: key, Value: value})
	}
	sort.SliceStable(buildArgs, func(i, j int) bool {
		return strings.Compare(buildArgs[i].Name, buildArgs[j].Name) < 0
	})
	*a = buildArgs
	return nil
}

func getArgs(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []Arg
	err := unmarshal(&rawList)
	if err == nil {
		for _, buildArg := range rawList {
			value, err := env.ExpandEnvIfNotEmpty(buildArg.Value)
			if err != nil {
				return nil, err
			}
			result[buildArg.Name] = value
		}
		return result, nil
	}
	var rawMap map[string]string
	err = unmarshal(&rawMap)
	if err != nil {
		return nil, err
	}
	for key, value := range rawMap {
		result[key], err = env.ExpandEnvIfNotEmpty(value)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// SerializeArgs returns build args as a list of strings
func SerializeArgs(buildArgs Args) []string {
	result := []string{}
	for _, e := range buildArgs {
		result = append(result, e.String())
	}
	// // stable serialization
	sort.Strings(result)
	return result
}
