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

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestUnmarshalExportCache(t *testing.T) {
	tests := []struct {
		expected *ExportCache
		name     string
		data     []byte
	}{
		{
			name: "single line",
			data: []byte(`"okteto/okteto:cache"`),
			expected: &ExportCache{
				"okteto/okteto:cache",
			},
		},
		{
			name: "export cache list",
			data: []byte(`- "okteto/okteto:cache"
- "okteto/test:cache"`),
			expected: &ExportCache{
				"okteto/okteto:cache",
				"okteto/test:cache",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *ExportCache
			err := yaml.UnmarshalStrict(tt.data, &result)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ExportCacheUnmarshalYAML_WithError(t *testing.T) {
	var ec ExportCache
	err := yaml.Unmarshal([]byte("some: invalid: yaml"), &ec)
	assert.Error(t, err)
}

func Test_ExportCacheMarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		ec       *ExportCache
		expected string
	}{
		{
			name:     "empty",
			ec:       &ExportCache{},
			expected: "[]\n",
		},
		{
			name:     "one image",
			ec:       &ExportCache{"test-registry/test-image:cache"},
			expected: "test-registry/test-image:cache\n",
		},
		{
			name:     "two images",
			ec:       &ExportCache{"test-registry/test-image-1:cache", "test-registry/test-image-2:cache"},
			expected: "- test-registry/test-image-1:cache\n- test-registry/test-image-2:cache\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yaml.Marshal(tt.ec)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
