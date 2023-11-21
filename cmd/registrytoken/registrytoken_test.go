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

package registrytoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRegistryCredentialHelperCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected bool
	}{
		{
			name:     "registrytoken command with get action",
			input:    []string{"foo", "registrytoken", "get"},
			expected: true,
		},
		{
			name:     "registrytoken command without action",
			input:    []string{"bar", "registrytoken"},
			expected: false,
		},
		{
			name:     "registrytoken command with flag",
			input:    []string{"bar", "registrytoken", "--help"},
			expected: false,
		},
		{
			name:     "non registrytoken command",
			input:    []string{"bar", "namespaces", "list"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRegistryCredentialHelperCommand(tt.input))
		})
	}
}
