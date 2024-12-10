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

package schema

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_withManifestRefDocLink(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		anchor   string
		expected string
	}{
		{
			name:     "empty",
			text:     "",
			anchor:   "",
			expected: "",
		},
		{
			name:     "empty anchor",
			text:     "text",
			anchor:   "",
			expected: "text",
		},
		{
			name:     "with anchor",
			text:     "text",
			anchor:   "anchor",
			expected: "text\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#anchor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, withManifestRefDocLink(tt.text, tt.anchor))
		})
	}
}
