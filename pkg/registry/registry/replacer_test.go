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

package registry

import (
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func TestReplace(t *testing.T) {
	type input struct {
		image        string
		ns           string
		registryType string
		registryURL  string
	}
	var tests = []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "okteto dev",
			input: input{
				image:        "okteto.dev/hello",
				ns:           "test",
				registryType: constants.DevRegistry,
				registryURL:  "my-registry.com",
			},
			expected: "my-registry.com/test/hello",
		},
		{
			name: "okteto global",
			input: input{
				image:        "okteto.global/hello",
				ns:           "global",
				registryType: constants.GlobalRegistry,
				registryURL:  "my-registry.com",
			},
			expected: "my-registry.com/global/hello",
		},
		{
			name: "not at the beginning",
			input: input{
				image:        "hello/okteto.dev",
				ns:           "test",
				registryType: constants.DevRegistry,
				registryURL:  "my-registry.com",
			},
			expected: "hello/okteto.dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replacer := NewRegistryReplacer(tt.input.registryURL)

			result := replacer.Replace(tt.input.image, tt.input.registryType, tt.input.ns)
			assert.Equal(t, tt.expected, result)
		})
	}

}
