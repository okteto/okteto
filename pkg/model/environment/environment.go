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
	"fmt"

	"github.com/a8m/envsubst"
)

// Environment is a list of environment variables (key, value pairs).
type Environment []EnvVar

// EnvVar represents an environment value. When loaded, it will expand from the current env
type EnvVar struct {
	Name  string `yaml:"name,omitempty"`
	Value string `yaml:"value,omitempty"`
}

//ExpandEnv expands the environments supporting the notation "${var:-$DEFAULT}"
func ExpandEnv(value string) (string, error) {
	result, err := envsubst.String(value)
	if err != nil {
		return "", fmt.Errorf("error expanding environment on '%s': %s", value, err.Error())
	}
	return result, nil
}

//SerializeBuildArgs returns build  aaargs as a llist of strings
func SerializeBuildArgs(buildArgs Environment) []string {
	result := []string{}
	for _, e := range buildArgs {
		result = append(
			result,
			fmt.Sprintf("%s=%s", e.Name, e.Value),
		)
	}
	return result
}
