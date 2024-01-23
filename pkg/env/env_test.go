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
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func Test_ExpandEnv(t *testing.T) {
	t.Setenv("BAR", "bar")
	tests := []struct {
		expectedErr error
		name        string
		result      string
		value       string
	}{
		{
			name:        "broken var - missing closing curly bracket",
			value:       "value-${BAR",
			result:      "",
			expectedErr: &VarExpansionErr{},
		},
		{
			name:        "no-var",
			value:       "value",
			result:      "value",
			expectedErr: nil,
		},
		{
			name:        "var",
			value:       "value-${BAR}-value",
			result:      "value-bar-value",
			expectedErr: nil,
		},
		{
			name:        "default",
			value:       "value-${FOO:-foo}-value",
			result:      "value-foo-value",
			expectedErr: nil,
		},
		{
			name:        "only bar expanded",
			value:       "${BAR}",
			result:      "bar",
			expectedErr: nil,
		},
		{
			name:        "only bar not expand if empty",
			value:       "${FOO}",
			result:      "",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandEnv(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr != nil {
				assert.ErrorAs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ExpandEnvIfNotEmpty(t *testing.T) {
	t.Setenv("BAR", "bar")
	tests := []struct {
		expectedErr error
		name        string
		result      string
		value       string
	}{
		{
			name:        "broken var - missing closing curly bracket",
			value:       "value-${BAR",
			result:      "",
			expectedErr: &VarExpansionErr{},
		},
		{
			name:        "no-var",
			value:       "value",
			result:      "value",
			expectedErr: nil,
		},
		{
			name:        "var",
			value:       "value-${BAR}-value",
			result:      "value-bar-value",
			expectedErr: nil,
		},
		{
			name:        "default",
			value:       "value-${FOO:-foo}-value",
			result:      "value-foo-value",
			expectedErr: nil,
		},
		{
			name:        "only bar expanded",
			value:       "${BAR}",
			result:      "bar",
			expectedErr: nil,
		},
		{
			name:        "only bar not expand if empty",
			value:       "${FOO}",
			result:      "${FOO}",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandEnvIfNotEmpty(tt.value)
			assert.Equal(t, tt.result, result)
			if tt.expectedErr != nil {
				assert.ErrorAs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Env_UnmarshalYAML(t *testing.T) {
	t.Setenv("VALUE", "test")
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
			name: "deserialized successfully with env var",
			yaml: []byte(`
foo: bar
unit: "unit-$VALUE"`),
			expected: Environment{
				{Name: "foo", Value: "bar"},
				{Name: "unit", Value: "unit-test"},
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

func TestLoadBooleanOrDefault(t *testing.T) {
	type tc struct {
		Name           string
		EnvKey         string
		EnvValue       string
		DefaultValue   bool
		ExpectedResult bool
	}

	testCases := []tc{
		{
			Name:           "Environment variable is 'true'",
			EnvKey:         "TEST_KEY",
			EnvValue:       "true",
			DefaultValue:   false,
			ExpectedResult: true,
		},
		{
			Name:           "Environment variable is 'false'",
			EnvKey:         "TEST_KEY",
			EnvValue:       "false",
			DefaultValue:   true,
			ExpectedResult: false,
		},
		{
			Name:           "Environment variable is not defined",
			EnvKey:         "TEST_KEY",
			EnvValue:       "",
			DefaultValue:   true,
			ExpectedResult: true,
		},
		{
			Name:           "Environment variable has an invalid value",
			EnvKey:         "TEST_KEY",
			EnvValue:       "invalid",
			DefaultValue:   false,
			ExpectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Setenv(tc.EnvKey, tc.EnvValue)
			result := LoadBooleanOrDefault(tc.EnvKey, tc.DefaultValue)
			assert.Equal(t, tc.ExpectedResult, result)
		})
	}
}
