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

func Test_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		expected interface{}
		name     string
		data     []byte
	}{
		{
			name:     "empty",
			data:     []byte("[]"),
			expected: From{},
		},
		{
			name:     "one single item",
			data:     []byte("test-registry/test-image:cache"),
			expected: From{"test-registry/test-image:cache"},
		},
		{
			name:     "one item as list",
			data:     []byte(`["test-registry/test-image:cache"]`),
			expected: From{"test-registry/test-image:cache"},
		},
		{
			name:     "one item as list",
			data:     []byte(`["test-registry/test-image:cache"]`),
			expected: From{"test-registry/test-image:cache"},
		},
		{
			name: "two items",
			data: []byte(`
- test-registry/test-image:1.0.0
- test-registry/another-image:1.0.0`),
			expected: From{"test-registry/test-image:1.0.0", "test-registry/another-image:1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cf From
			err := yaml.Unmarshal(tt.data, &cf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, cf)
		})
	}
}

func Test_UnmarshalYAML_WithError(t *testing.T) {
	var cf From

	err := cf.UnmarshalYAML(func(interface{}) error {
		return assert.AnError
	})

	assert.ErrorIs(t, err, assert.AnError)
}

func Test_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		cf       *From
		expected string
	}{
		{
			name:     "empty",
			cf:       &From{},
			expected: "[]\n",
		},
		{
			name:     "one image",
			cf:       &From{"test-registry/test-image:cache"},
			expected: "test-registry/test-image:cache\n",
		},
		{
			name:     "two images",
			cf:       &From{"test-registry/test-image-1:cache", "test-registry/test-image-2:cache"},
			expected: "- test-registry/test-image-1:cache\n- test-registry/test-image-2:cache\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yaml.Marshal(tt.cf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
