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

func TestEnvFileUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Files
	}{
		{
			"single value",
			[]byte(`.testEnv`),
			Files{".testEnv"},
		},
		{
			"testEnv files list",
			[]byte("\n  - .testEnv\n  - .env2"),
			Files{".testEnv", ".env2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(Files, 0)

			err := yaml.UnmarshalStrict(tt.data, &result)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
