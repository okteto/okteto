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

	"github.com/okteto/okteto/pkg/env"
	"github.com/spf13/afero"
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
		name        string
		expected    string
		vars        Vars
		expectedErr bool
	}{
		{
			name: "serialized successfully",
			vars: Vars{
				{
					Name:  "foo1",
					Value: "bar1",
				},
				{
					Name:  "foo2",
					Value: "bar2",
				},
				{
					Name:  "foo3",
					Value: "bar3",
				},
			},
			expected: "foo1: bar1\nfoo2: bar2\nfoo3: bar3\n",
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

func Test_ExpandSuccess(t *testing.T) {
	t.Setenv("UNIT_TEST_VALUE1", "value-1")

	vars := Vars{
		Var{Name: "UNIT_TEST_VAR1", Value: "$UNIT_TEST_VALUE1"},
		Var{Name: "UNIT_TEST_VAR2", Value: "value-2"},
		Var{Name: "UNIT_TEST_VAR3", Value: "$UNIT_TEST_VALUE3"},
	}

	err := vars.Expand(env.ExpandEnvIfNotEmpty)
	assert.NoError(t, err)

	assert.Equal(t, "value-1", vars[0].Value)
	assert.Equal(t, "value-2", vars[1].Value)
	assert.Equal(t, "$UNIT_TEST_VALUE3", vars[2].Value)
}

func Test_GetManifestVarsSuccess(t *testing.T) {
	manifest := `variables:
  UNIT_TEST_VAR1: value-1
  UNIT_TEST_VAR2: value-2
  UNIT_TEST_VAR3: value-3`

	fs := afero.NewMemMapFs()
	manifestPath := "/okteto.yml"
	err := afero.WriteFile(fs, manifestPath, []byte(manifest), 0644)
	assert.NoError(t, err)

	vars, err := GetManifestVars(manifestPath, fs)
	assert.NoError(t, err)

	// helper func to find a var by name
	findVar := func(name string) Var {
		for _, v := range vars {
			if v.Name == name {
				return v
			}
		}
		return Var{}
	}

	assert.Equal(t, "value-1", findVar("UNIT_TEST_VAR1").Value)
	assert.Equal(t, "value-2", findVar("UNIT_TEST_VAR2").Value)
	assert.Equal(t, "value-3", findVar("UNIT_TEST_VAR3").Value)
	assert.Equal(t, "", findVar("NON_EXISTING_VAR").Value)
}

func Test_GetManifestVarsUnmarshallingFail(t *testing.T) {
	manifest := `variables:
  - UNIT_TEST_VAR1: value-1
  - UNIT_TEST_VAR2: value-2
  - UNIT_TEST_VAR3: value-3`

	fs := afero.NewMemMapFs()
	manifestPath := "/okteto.yml"
	err := afero.WriteFile(fs, manifestPath, []byte(manifest), 0644)
	assert.NoError(t, err)

	vars, err := GetManifestVars(manifestPath, fs)
	assert.Error(t, err)
	assert.Nil(t, vars)
}

func Test_GetManifestVarsReadFileFail(t *testing.T) {
	fs := afero.NewMemMapFs()
	manifestPath := "/okteto.yml"

	vars, err := GetManifestVars(manifestPath, fs)
	assert.ErrorIs(t, err, afero.ErrFileNotFound)
	assert.Nil(t, vars)
}
