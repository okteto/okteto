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
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/validator"
)

// validateAndSet returns error when variables dont have expected format NAME=VALUE or NAME is not allowed
// when variable is valid, it sets its value as env variable
func validateAndSet(variables []string, setEnv func(key, value string) error) error {
	if err := validator.CheckReservedVariablesNameOption(variables); err != nil {
		return err
	}

	envVars, err := env.Parse(variables)
	if err != nil {
		return err
	}
	return setOptionVarsAsEnvs(envVars, setEnv)
}

func setOptionVarsAsEnvs(variables []env.Var, setEnv func(key, value string) error) error {
	for _, v := range variables {
		if err := setEnv(v.Name, v.Value); err != nil {
			return err
		}
	}
	return nil
}
