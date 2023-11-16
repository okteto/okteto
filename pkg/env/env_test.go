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
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"testing"
)

func Test_ExpandEnv(t *testing.T) {
	t.Setenv("BAR", "bar")
	tests := []struct {
		expectedErr   error
		name          string
		result        string
		value         string
		expandIfEmpty bool
	}{
		{
			name:          "broken var - missing closing curly bracket",
			value:         "value-${BAR",
			expandIfEmpty: true,
			result:        "",
			expectedErr:   fmt.Errorf("error expanding environment on 'value-${BAR': closing brace expected"),
		},
		{
			name:          "no-var",
			value:         "value",
			expandIfEmpty: true,
			result:        "value",
			expectedErr:   nil,
		},
		{
			name:          "var",
			value:         "value-${BAR}-value",
			expandIfEmpty: true,
			result:        "value-bar-value",
			expectedErr:   nil,
		},
		{
			name:          "default",
			value:         "value-${FOO:-foo}-value",
			expandIfEmpty: true,
			result:        "value-foo-value",
			expectedErr:   nil,
		},
		{
			name:          "only bar expanded",
			value:         "${BAR}",
			expandIfEmpty: true,
			result:        "bar",
			expectedErr:   nil,
		},
		{
			name:          "only bar not expand if empty",
			value:         "${FOO}",
			expandIfEmpty: false,
			result:        "${FOO}",
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandEnv(tt.value, tt.expandIfEmpty)
			assert.Equal(t, err, tt.expectedErr)
			assert.Equal(t, tt.result, result)
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
