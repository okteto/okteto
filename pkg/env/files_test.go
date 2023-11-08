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
	"gopkg.in/yaml.v2"
	"reflect"
	"testing"
)

func TestEnvFileUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected EnvFiles
	}{
		{
			"single value",
			[]byte(`.testEnv`),
			EnvFiles{".testEnv"},
		},
		{
			"testEnv files list",
			[]byte("\n  - .testEnv\n  - .env2"),
			EnvFiles{".testEnv", ".env2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(EnvFiles, 0)

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}
