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
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"testing"
)

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
			expected: "'foo: bar'\n",
		},
		{
			name: "empty name - serialized successfully",
			v: Var{
				Name:  "",
				Value: "bar",
			},
			expected: "': bar'\n",
		},
		{
			name: "empty - serialized successfully",
			v: Var{
				Name:  "foo",
				Value: "",
			},
			expected: "'foo: '\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := yaml.Marshal(tt.v)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(b))
			}
		})
	}
}
