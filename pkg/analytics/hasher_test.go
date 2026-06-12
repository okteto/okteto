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

package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_hashString(t *testing.T) {
	input := "test-string"
	require.Equal(t, hashString(input), "ffe65f1d98fafedea3514adc956c8ada5980c6c5d2552fd61f48401aefd5c00e")
}

func Test_normalizeRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized https",
			input:    "https://github.com/org/repo",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "http converted to https",
			input:    "http://github.com/org/repo",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "trailing .git removed",
			input:    "https://github.com/org/repo.git",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "http with .git",
			input:    "http://github.com/org/repo.git",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "ssh git@ converted to https",
			input:    "git@github.com:org/repo.git",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "ssh git@ without .git",
			input:    "git@github.com:org/repo",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "uppercased url lowercased",
			input:    "https://GitHub.COM/Org/Repo",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "mixed case with .git",
			input:    "HTTPS://GitHub.com/Org/Repo.git",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "trailing slash removed",
			input:    "https://github.com/org/repo/",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "trailing slash and .git removed",
			input:    "https://github.com/org/repo.git/",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "credentials stripped from https url",
			input:    "https://user:password@github.com/org/repo",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "query params stripped",
			input:    "https://github.com/org/repo?token=secret&ref=main",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "fragment stripped",
			input:    "https://github.com/org/repo#readme",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "ssh:// scheme converted to https",
			input:    "ssh://git@github.com/org/repo.git",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "credentials and query params stripped together",
			input:    "https://user:token@github.com/org/repo.git?ref=main#section",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "gitlab subgroup ssh",
			input:    "git@gitlab.com:group/subgroup/repo.git",
			expected: "https://gitlab.com/group/subgroup/repo",
		},
		{
			name:     "gitlab subgroup https",
			input:    "https://gitlab.com/group/subgroup/repo.git",
			expected: "https://gitlab.com/group/subgroup/repo",
		},
		{
			name:     "gitlab deep nesting",
			input:    "git@gitlab.com:a/b/c/d/repo.git",
			expected: "https://gitlab.com/a/b/c/d/repo",
		},
		{
			name:     "bitbucket ssh",
			input:    "git@bitbucket.org:workspace/repo.git",
			expected: "https://bitbucket.org/workspace/repo",
		},
		{
			name:     "bitbucket https",
			input:    "https://bitbucket.org/workspace/repo.git",
			expected: "https://bitbucket.org/workspace/repo",
		},
		{
			name:     "git:// protocol converted to https",
			input:    "git://github.com/org/repo.git",
			expected: "https://github.com/org/repo",
		},
		{
			name:     "git:// protocol gitlab subgroup",
			input:    "git://gitlab.com/group/subgroup/repo.git",
			expected: "https://gitlab.com/group/subgroup/repo",
		},
		{
			name:     "file:// local path returned as-is lowercased",
			input:    "file:///path/to/repo",
			expected: "file:///path/to/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, normalizeRepoURL(tt.input))
		})
	}
}
