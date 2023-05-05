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

package deploy

import (
	"fmt"
	"strings"
)

type envVar struct {
	key   string
	value string
}

func validateAndSetVariablesAsEnvs(variables []string, setEnv func(key, value string) error) error {
	envVars, err := parse(variables)
	if err != nil {
		return err
	}
	return setOptionVarsAsEnvs(envVars, setEnv)
}

func getVariablesWithoutEscapedValues(variables []string) []string {
	cleanedVariables := []string{}
	for _, v := range variables {
		cleanedVariables = append(cleanedVariables, strings.ReplaceAll(v, "\"", ""))
	}
	return cleanedVariables
}

func parse(variables []string) ([]envVar, error) {
	result := []envVar{}
	for _, v := range variables {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		result = append(result, envVar{key: kv[0], value: kv[1]})
	}
	return result, nil
}

func setOptionVarsAsEnvs(variables []envVar, setEnv func(key, value string) error) error {
	for _, v := range variables {
		if err := setEnv(v.key, v.value); err != nil {
			return err
		}
	}
	return nil
}
