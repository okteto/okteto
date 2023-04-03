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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	yaml "gopkg.in/yaml.v2"
)

type mockRegistry struct {
	imageCtrl ImageCtrlInterface
}

func (m *mockRegistry) GetImageCtrl() ImageCtrlInterface {
	return m.imageCtrl
}

func (*mockRegistry) HasGlobalPushAccess() (bool, error) {
	return true, nil
}

type mockRegistryWithError struct {
	imageCtrl ImageCtrlInterface
}

func (m *mockRegistryWithError) GetImageCtrl() ImageCtrlInterface {
	return m.imageCtrl
}

func (*mockRegistryWithError) HasGlobalPushAccess() (bool, error) {
	return false, assert.AnError
}

type mockImageCtrl struct{}

func (*mockImageCtrl) GetRegistryAndRepo(_ string) (string, string) {
	return "registry", "test-account/test-image:x.y.z"
}

func (*mockImageCtrl) GetRepoNameAndTag(_ string) (string, string) {
	return "test-account/test-image", "x.y.z"
}

func Test_AddDefaultPullCache(t *testing.T) {
	imageCtrl := &mockImageCtrl{}
	reg := &mockRegistry{imageCtrl: imageCtrl}

	tests := []struct {
		name     string
		image    string
		cf       CacheFrom
		expected CacheFrom
	}{
		{
			name:     "already with cache image",
			image:    "test-account/test-image:1.0.0",
			cf:       CacheFrom{"okteto.global/test-account/test-image:cache", "okteto.dev/test-account/test-image:cache"},
			expected: CacheFrom{"okteto.global/test-account/test-image:cache", "okteto.dev/test-account/test-image:cache"},
		},
		{
			name:     "without cache image",
			image:    "test-account/test-image:1.0.0",
			cf:       CacheFrom{},
			expected: CacheFrom{"okteto.global/test-account/test-image:cache", "okteto.dev/test-account/test-image:cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cf.AddDefaultPullCache(reg, tt.image)
			assert.Equal(t, tt.expected, tt.cf)
		})
	}
}

func Test_AddDefaultPullCache_WithError(t *testing.T) {
	image := "test-account/test-image:x.y.z"
	cf := CacheFrom{}
	expected := CacheFrom{"okteto.dev/test-account/test-image:cache"}

	imageCtrl := &mockImageCtrl{}
	registry := &mockRegistryWithError{imageCtrl: imageCtrl}

	cf.AddDefaultPullCache(registry, image)

	assert.Equal(t, cf, expected)
}

func Test_hasCacheFromImage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cf       CacheFrom
		expected bool
	}{
		{
			name:     "not found",
			input:    "test-registry/test-image:cache",
			cf:       CacheFrom{"test-registry/test-image:test-tag"},
			expected: false,
		},
		{
			name:     "found",
			input:    "test-registry/test-image:cache",
			cf:       CacheFrom{"test-registry/test-image:cache"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cf.hasCacheFromImage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected interface{}
	}{
		{
			name:     "empty",
			data:     []byte("[]"),
			expected: CacheFrom{},
		},
		{
			name:     "one item",
			data:     []byte(`["test-registry/test-image:cache"]`),
			expected: CacheFrom{"test-registry/test-image:cache"},
		},
		{
			name: "two items",
			data: []byte(`
- test-registry/test-image:1.0.0
- test-registry/another-image:1.0.0`),
			expected: CacheFrom{"test-registry/test-image:1.0.0", "test-registry/another-image:1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cf CacheFrom
			err := yaml.Unmarshal([]byte(tt.data), &cf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, cf)
		})
	}
}

func Test_UnmarshalYAML_WithError(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectedErr error
	}{
		{
			name:        "invalid type",
			data:        []byte("some: invalid: yaml"),
			expectedErr: fmt.Errorf("yaml: mapping values are not allowed in this context"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cf CacheFrom
			err := yaml.Unmarshal([]byte(tt.data), &cf)
			assert.Error(t, err)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func Test_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		cf       CacheFrom
		expected string
	}{
		{
			name:     "empty",
			cf:       CacheFrom{},
			expected: "[]\n",
		},
		{
			name:     "one image",
			cf:       CacheFrom{"test-registry/test-image:cache"},
			expected: "test-registry/test-image:cache\n",
		},
		{
			name:     "two images",
			cf:       CacheFrom{"test-registry/test-image-1:cache", "test-registry/test-image-2:cache"},
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
