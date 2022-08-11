// Copyright 2022 The Okteto Authors
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
	"os"
	"strings"
)

func validateAndSet(variables []string, setEnv func(key, value string) error) error {

	if err := validateOptionVars(variables); err != nil {
		return err
	}
	return setOptionVarsAsEnvs(variables, setEnv)
}

func validateOptionVars(variables []string) error {
	for _, v := range variables {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

func setOptionVarsAsEnvs(variables []string, setEnv func(key, value string) error) error {
	for _, v := range variables {
		kv := strings.SplitN(v, "=", 2)
		if err := setEnv(kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}
