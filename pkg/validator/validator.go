// Copyright 2024 The Okteto Authors
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

package validator

import (
	"errors"
	"strings"
)

// errForbiddenVariableName is raised when a variable from cmd option has invalid name
var errForbiddenVariableName = errors.New("variable name not allowed: <placeholder>")

// CheckForbiddenVariablesNameOption returns an error when any of the variable names from command flags are not allowed as input
func CheckForbiddenVariablesNameOption(variables []string) error {
	for _, v := range variables {
		name, _, ok := strings.Cut(v, "=")
		if ok && isForbiddenVariableName(name) {
			return errForbiddenVariableName
		}
	}
	return nil
}

// isForbiddenVariableName returns true when variable name is not allowed
func isForbiddenVariableName(name string) bool {
	forbidden := map[string]bool{
		"OKTETO_CONTEXT":   true,
		"OKTETO_NAMESPACE": true,
		"OKTETO_URL":       true,
	}

	return forbidden[strings.ToUpper(name)]
}
