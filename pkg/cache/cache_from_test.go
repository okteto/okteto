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

// func Test_AddDefaultPullCache(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		input    *model.BuildInfo
// 		expected *model.BuildInfo
// 	}{
// 		{
// 			name: "already with cache image",
// 			input: &model.BuildInfo{
// 				Image:     "test-registry/test-account/test-image:1.0.0",
// 				CacheFrom: []string{"okteto.global/test-account/test-image:cache", "okteto.dev/test-account/test-image:cache"},
// 			},
// 			expected: &model.BuildInfo{
// 				Image:     "test-registry/test-account/test-image:1.0.0",
// 				CacheFrom: []string{"okteto.global/test-account/test-image:cache", "okteto.dev/test-account/test-image:cache"},
// 			},
// 		},
// 		{
// 			name: "without cache image",
// 			input: &model.BuildInfo{
// 				Name:  "test",
// 				Image: "test-registry/test-account/test-image:1.0.0",
// 			},
// 			expected: &model.BuildInfo{
// 				Name:      "test",
// 				Image:     "test-registry/test-account/test-image:1.0.0",
// 				CacheFrom: []string{"okteto.global/test-account/test-image:cache", "okteto.dev/test-account/test-image:cache"},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			AddDefaultPullCache(tt.input)
// 			assert.Equal(t, tt.expected, tt.input)
// 		})
// 	}
// }

// func Test_hasCacheFromImage(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		input    string
// 		manifest *model.BuildInfo
// 		expected bool
// 	}{
// 		{
// 			name:  "not found",
// 			input: "test-registry/test-image:cache",
// 			manifest: &model.BuildInfo{
// 				Name:      "test",
// 				CacheFrom: []string{"test-registry/test-image:test-tag"},
// 			},
// 			expected: false,
// 		},
// 		{
// 			name:  "found",
// 			input: "test-registry/test-image:cache",
// 			manifest: &model.BuildInfo{
// 				Name:      "test",
// 				CacheFrom: []string{"test-registry/test-image:cache"},
// 			},
// 			expected: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := hasCacheFromImage(tt.manifest, tt.input)
// 			assert.Equal(t, tt.expected, result)
// 		})
// 	}
// }
