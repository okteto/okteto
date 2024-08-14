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
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/stretchr/testify/assert"
)

type fakeVarManager struct{}

func (*fakeVarManager) MaskVar(string) {}
func (*fakeVarManager) IsLocalVarSupportEnabled() bool {
	return false
}
func (*fakeVarManager) IsLocalVarException(string) bool {
	return false
}

func Test_convertCommandFlagsToOktetoVariables(t *testing.T) {
	var tests = []struct {
		expectedError error
		expectedEnvs  []string
		name          string
		variables     []string
	}{
		{
			name:          "correct assignment",
			variables:     []string{"NAME=test"},
			expectedError: nil,
			expectedEnvs:  []string{"NAME=test"},
		},
		{
			name:          "bad assignment",
			variables:     []string{"NAME:test"},
			expectedError: fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", "NAME:test"),
			expectedEnvs:  []string{},
		},
		{
			name:          "more than 2 equals",
			variables:     []string{"too=many=equals"},
			expectedError: nil,
			expectedEnvs:  []string{"too=many=equals"},
		},
		{
			name: "multiple variables",
			variables: []string{
				"NAME=test",
				"BASE64=something==",
			},
			expectedError: nil,
			expectedEnvs:  []string{"NAME=test", "BASE64=something=="},
		},
		{
			name:      "reserved variable name",
			variables: []string{"OKTETO_CONTEXT=value"},
			expectedError: errors.UserError{
				E:    fmt.Errorf("%s is %w.", "OKTETO_CONTEXT", validator.ErrReservedVariableName),
				Hint: "See documentation for more info: https://www.okteto.com/docs/core/credentials/environment-variables/",
			},
			expectedEnvs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varManager := vars.NewVarsManager(&fakeVarManager{})
			err := convertCommandFlagsToOktetoVariables(tt.variables, varManager)

			assert.Equal(t, tt.expectedError, err)
			assert.True(t, reflect.DeepEqual(tt.expectedEnvs, varManager.GetOktetoVariablesExcLocal()))
		})
	}
}
