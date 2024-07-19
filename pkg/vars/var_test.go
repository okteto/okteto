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
	"gopkg.in/yaml.v2"
)

//
//type fakeVarManager struct{}
//
//func (*fakeVarManager) MaskVar(string)                     {}
//func (*fakeVarManager) WarningLogf(string, ...interface{}) {}

func Test_Var_UnmarshalYAML(t *testing.T) {
	GlobalVarManager = NewVarsManager(&fakeVarManager{})

	GlobalVarManager.AddLocalVar("LOCAL_VAR", "local-env-var")
	GlobalVarManager.AddDotEnvVar("DYNAMIC_VAR_VALUE", "test")
	GlobalVarManager.AddDotEnvVar("VALUE", "test")

	tests := []struct {
		expected    Var
		name        string
		yaml        []byte
		expectedErr bool
	}{
		{
			name: "deserialized successfully",
			yaml: []byte(`foo=bar`),
			expected: Var{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			name: "local vars are not expanded",
			yaml: []byte(`name=unit-$LOCAL_VAR`),
			expected: Var{
				Name:  "name",
				Value: "unit-",
			},
		},
		{
			name: "deserialized successfully with env var",
			yaml: []byte(`name=unit-$VALUE`),
			expected: Var{
				Name:  "name",
				Value: "unit-test",
			},
		},
		{
			name: "deserialized successfully using dynamic name",
			yaml: []byte(`DYNAMIC_VAR_VALUE`),
			expected: Var{
				Name:  "DYNAMIC_VAR_VALUE",
				Value: "test",
			},
		},
		{
			name:        "fail to deserialize",
			yaml:        []byte(`- foo`),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v Var
			err := yaml.Unmarshal(tt.yaml, &v)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, v)
				assert.Equal(t, tt.expected.String(), v.String())
			}
		})
	}
}

func Test_Var_MarshalYAML(t *testing.T) {
	tests := []struct {
		v           Var
		name        string
		expected    string
		expectedErr bool
	}{
		{
			name: "serialized successfully",
			v: Var{
				Name:  "foo",
				Value: "bar",
			},
			expected: "foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := tt.v.MarshalYAML()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, out)
			}
		})
	}
}

func Test_CreateGroupFromLocalVars(t *testing.T) {
	t.Run("create Group from local vars", func(t *testing.T) {
		fakeLocalVars := []string{
			"TEST_VAR_1=local-value1",
			"TEST_VAR_2=local-value2",
		}

		fakeLocalGroup := ConvertLocalEnvVarsToOktetoVars(func() []string {
			return fakeLocalVars
		})

		assert.Equal(t, 2, len(fakeLocalGroup))
		assert.ElementsMatch(t, []Var{
			{
				Name:  "TEST_VAR_1",
				Value: "local-value1",
			},
			{
				Name:  "TEST_VAR_2",
				Value: "local-value2",
			},
		}, fakeLocalGroup)
	})
}
