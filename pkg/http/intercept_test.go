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

package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterceptAppend(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "nothing",
		},
		{
			name:     "single item",
			input:    []string{"https://example.com"},
			expected: []string{"example.com"},
		},
		{
			name:     "multiple items",
			input:    []string{"https://example.com", "https://foo.dev"},
			expected: []string{"example.com", "foo.dev"},
		},
		{
			name:     "repeated items",
			input:    []string{"https://example.com/alice", "https://foo.dev", "https://example.com/bob"},
			expected: []string{"example.com", "foo.dev"},
		},
		{
			name:     "multiple items",
			input:    []string{"https://example.com", "https://foo.dev"},
			expected: []string{"example.com", "foo.dev"},
		},
		{
			name:     "an invalid item",
			input:    []string{"https://example.com", "oneTwOthReE", "https://foo.dev"},
			expected: []string{"example.com", "foo.dev"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Intercept{}
			i.AppendURLs(tt.input...)

			r := []string{}
			for k := range i {
				r = append(r, k)
			}
			assert.ElementsMatch(t, tt.expected, r)
		})
	}
}

func TestInterceptMatch(t *testing.T) {
	tests := []struct {
		expected  map[string]bool
		name      string
		bootstrap []string
	}{
		{
			name:      "multiple items",
			bootstrap: []string{"https://example.com", "https://foo.dev"},
			expected:  map[string]bool{"example.com:443": true, "foo.dev:443": true, "foo.dev:445": true, "missing.port": false, "other.com:443": false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Intercept{}
			i.AppendURLs(tt.bootstrap...)

			for k, v := range tt.expected {
				assert.Equal(t, v, i.ShouldInterceptAddr(k), k)
			}
		})
	}
}
