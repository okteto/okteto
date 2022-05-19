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

type envKeyValue struct {
	key   string
	value string
}

func setEnvVars(variables []string) error {
	var varsToSet []envKeyValue
	for _, v := range variables {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		varsToSet = append(varsToSet, envKeyValue{
			key:   kv[0],
			value: kv[1],
		})
	}

	for _, each := range varsToSet {
		if err := os.Setenv(each.key, each.value); err != nil {
			return err
		}
	}
	return nil
}
