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

package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_Vars_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		expected    Vars
		name        string
		yaml        []byte
		expectedErr bool
	}{
		{
			name: "failed to deserialize",
			yaml: []byte(`
UNIT_TEST_VAR_1=value1
`),
			expectedErr: true,
		},
		{
			name: "deserialized single var successfully",
			yaml: []byte(`
UNIT_TEST_VAR_1: value1
`),
			expected: Vars{
				{
					Name:  "UNIT_TEST_VAR_1",
					Value: "value1",
				},
			},
		},
		{
			name: "deserialized multiple vars successfully",
			yaml: []byte(`
UNIT_TEST_VAR_1: value1
UNIT_TEST_VAR_2: value2
UNIT_TEST_VAR_3: value3
`),
			expected: Vars{
				{
					Name:  "UNIT_TEST_VAR_1",
					Value: "value1",
				},
				{
					Name:  "UNIT_TEST_VAR_2",
					Value: "value2",
				},
				{
					Name:  "UNIT_TEST_VAR_3",
					Value: "value3",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vars Vars
			err := yaml.Unmarshal(tt.yaml, &vars)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.ElementsMatch(t, tt.expected, vars)
			}
		})
	}
}

func Test_Vars_MarshalYAML(t *testing.T) {
	tests := []struct {
		vars        Vars
		name        string
		expected    string
		expectedErr bool
	}{
		{
			name: "serialized successfully",
			vars: Vars{
				{
					Name:  "foo",
					Value: "bar",
				},
			},
			expected: "foo: bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := yaml.Marshal(tt.vars)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(b))
			}
		})
	}
}
