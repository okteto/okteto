// Copyright 2026 The Okteto Authors
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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_isOktetoRepoFromURL_endpointForms(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "scp-like without user from okteto",
			url:      "github.com:okteto/movies.git",
			expected: true,
		},
		{
			name:     "scp-like without user not from okteto",
			url:      "github.com:test/movies.git",
			expected: false,
		},
		{
			name:     "scp-like with custom user from okteto",
			url:      "deploy@gitlab.example.com:okteto/app.git",
			expected: true,
		},
		{
			name:     "scp-like with nested path not from okteto",
			url:      "git@gitlab.example.com:group/okteto/app.git",
			expected: false,
		},
		{
			name:     "ssh url with port from okteto",
			url:      "ssh://git@github.com:22/okteto/test.git",
			expected: true,
		},
		{
			name:     "https url with credentials from okteto",
			url:      "https://user:token@github.com/okteto/test.git",
			expected: true,
		},
		{
			name:     "https url with port not from okteto",
			url:      "https://git.example.com:8443/test/okteto.git",
			expected: false,
		},
		{
			name:     "git protocol url from okteto",
			url:      "git://github.com/okteto/test.git",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, isOktetoRepoFromURL(tt.url))
		})
	}
}
