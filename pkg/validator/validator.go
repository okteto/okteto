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
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

// ErrReservedVariableName is raised when a variable from cmd option has invalid name
var ErrReservedVariableName = errors.New("reserved variable name")

// CheckReservedVariablesNameOption returns an error when any of the variable names from command flags are not allowed as input
func CheckReservedVariablesNameOption(variables []string) error {
	for _, v := range variables {
		name, _, ok := strings.Cut(v, "=")
		if ok && isReservedVariableName(name) {
			return oktetoErrors.UserError{
				E:    fmt.Errorf("%s is %w.", name, ErrReservedVariableName),
				Hint: "See documentation for more info: https://www.okteto.com/docs/core/credentials/environment-variables/",
			}
		}
	}
	return nil
}

// CheckReservedVarName returns an error when any of the variable names from dependency manifest
func CheckReservedVarName(variables []env.Var) error {
	for _, v := range variables {
		if isReservedVariableName(v.Name) {
			return fmt.Errorf("%s is %w", v.Name, ErrReservedVariableName)
		}
	}
	return nil
}

// isReservedVariableName returns true when variable name is not allowed
func isReservedVariableName(name string) bool {
	reserved := map[string]bool{
		"OKTETO_CONTEXT":   true,
		"OKTETO_NAMESPACE": true,
		"OKTETO_URL":       true,
		"OKTETO_TOKEN":     true,
	}

	return reserved[strings.ToUpper(name)]
}
