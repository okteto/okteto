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

	"github.com/okteto/okteto/pkg/env"
)

func validateAndSetVarsFromFlag(variables []string, envManager *env.Manager) error {
	envVars, err := parse(variables)
	if err != nil {
		return err
	}
	envManager.AddGroup(envVars, env.PriorityVarFromFlag)
	return envManager.Export()
}

func parse(variables []string) ([]env.Var, error) {
	var result []env.Var
	for _, v := range variables {
		variableFormatParts := 2
		kv := strings.SplitN(v, "=", variableFormatParts)
		if len(kv) != variableFormatParts {
			return nil, fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		result = append(result, env.Var{Name: kv[0], Value: kv[1]})
	}
	return result, nil
}
