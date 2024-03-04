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
	"os"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/env"
	"github.com/stretchr/testify/assert"
)

type fakeEnvManager struct {
	envVarStorage map[string]string
}

func (e *fakeEnvManager) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
func (e *fakeEnvManager) SetEnv(key, value string) error {
	e.envVarStorage[key] = value
	return nil
}
func (e *fakeEnvManager) MaskVar(_ string) {}
func (e *fakeEnvManager) WarningLogf(_ string, _ ...interface{}) {
}

func newFakeEnvManager(envVarStorage map[string]string) *fakeEnvManager {
	return &fakeEnvManager{
		envVarStorage: envVarStorage,
	}
}

func Test_validateAndSet(t *testing.T) {
	var tests = []struct {
		expectedError error
		expectedEnvs  map[string]string
		name          string
		variables     []string
	}{
		{
			name:          "correct assignment",
			variables:     []string{"NAME=test"},
			expectedError: nil,
			expectedEnvs:  map[string]string{"NAME": "test"},
		},
		{
			name:          "bad assignment",
			variables:     []string{"NAME:test"},
			expectedError: fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", "NAME:test"),
			expectedEnvs:  map[string]string{},
		},
		{
			name:          "more than 2 equals",
			variables:     []string{"too=many=equals"},
			expectedError: nil,
			expectedEnvs:  map[string]string{"too": "many=equals"},
		},
		{
			name: "multiple variables",
			variables: []string{
				"NAME=test",
				"BASE64=something==",
			},
			expectedError: nil,
			expectedEnvs:  map[string]string{"NAME": "test", "BASE64": "something=="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVarStorage := make(map[string]string)
			envManager := env.NewEnvManager(newFakeEnvManager(envVarStorage))
			err := validateAndSetVarsFromFlag(tt.variables, envManager)

			assert.Equal(t, tt.expectedError, err)
			assert.True(t, reflect.DeepEqual(tt.expectedEnvs, envVarStorage))
		})
	}
}
