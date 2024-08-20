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

package env

import (
	"github.com/okteto/okteto/pkg/vars"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"testing"
)

type varManagerLogger struct{}

func (varManagerLogger) Yellow(_ string, _ ...interface{}) {}
func (varManagerLogger) AddMaskedWord(_ string)            {}

func Test_Env_UnmarshalYAML(t *testing.T) {
	vars.GlobalVarManager = vars.NewVarsManager(&varManagerLogger{})
	vars.GlobalVarManager.AddLocalVar("LOCAL_VAR", "local-var")
	vars.GlobalVarManager.AddFlagVar("FLAG_VAR", "flag-var")

	tests := []struct {
		expected    Environment
		name        string
		yaml        []byte
		expectedErr bool
	}{
		{
			name: "deserialized successfully",
			yaml: []byte(`
foo: bar
unit: test`),
			expected: Environment{
				{Name: "foo", Value: "bar"},
				{Name: "unit", Value: "test"},
			},
		},
		{
			name: "deserialized successfully with local var",
			yaml: []byte(`
foo: bar
unit: "unit-$LOCAL_VAR"`),
			expected: Environment{
				{Name: "foo", Value: "bar"},
				{Name: "unit", Value: "unit-local-var"},
			},
		},
		{
			name: "deserialized successfully with flag var",
			yaml: []byte(`
foo: bar
unit: "unit-$FLAG_VAR"`),
			expected: Environment{
				{Name: "foo", Value: "bar"},
				{Name: "unit", Value: "unit-flag-var"},
			},
		},
		{
			name:        "fail to deserialize",
			yaml:        []byte(`foo`),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e Environment
			err := yaml.Unmarshal(tt.yaml, &e)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, e)
			}
		})
	}
}

func TestLoadBoolean(t *testing.T) {
	tests := []struct {
		name      string
		mockKey   string
		mockValue string
		key       string
		expected  bool
	}{
		{
			name:     "empty key",
			expected: false,
		},
		{
			name:     "empty value",
			mockKey:  "NON_EXISTING_VAR_UNIT_TEST",
			expected: false,
		},
		{
			name:      "false - string",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "random value",
			expected:  false,
		},
		{
			name:      "false - boolean",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "false",
			expected:  false,
		},
		{
			name:      "false - int",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "0",
			expected:  false,
		},
		{
			name:      "true - boolean",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "true",
			expected:  true,
		},
		{
			name:      "true - int",
			mockKey:   "VAR_UNIT_TEST",
			mockValue: "1",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockKey != "" {
				t.Setenv(tt.mockKey, tt.mockValue)
			}
			got := LoadBoolean(tt.mockKey)
			assert.Equal(t, tt.expected, got)
		})
	}
}
