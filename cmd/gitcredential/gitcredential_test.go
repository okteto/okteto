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

package gitcredential

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsGitCredentialHelperCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "get",
			args:     []string{"okteto", "git-credential", "get"},
			expected: true,
		},
		{
			name:     "store",
			args:     []string{"okteto", "git-credential", "store"},
			expected: true,
		},
		{
			name:     "erase",
			args:     []string{"okteto", "git-credential", "erase"},
			expected: true,
		},
		{
			name:     "get with a flag before it",
			args:     []string{"okteto", "git-credential", "-l", "debug", "get"},
			expected: true,
		},
		{
			name:     "get with a flag after it",
			args:     []string{"okteto", "git-credential", "get", "--log-level", "debug"},
			expected: true,
		},
		{
			name:     "install",
			args:     []string{"okteto", "git-credential", "install", "--system"},
			expected: false,
		},
		{
			name:     "without an action",
			args:     []string{"okteto", "git-credential"},
			expected: false,
		},
		{
			name:     "with a flag",
			args:     []string{"okteto", "git-credential", "--help"},
			expected: false,
		},
		{
			name:     "another command",
			args:     []string{"okteto", "registrytoken", "get"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, IsGitCredentialHelperCommand(tt.args))
		})
	}
}

func TestIsGitCredentialCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "get",
			args:     []string{"okteto", "git-credential", "get"},
			expected: true,
		},
		{
			name:     "install",
			args:     []string{"okteto", "git-credential", "install", "--system", "--log-output=json"},
			expected: true,
		},
		{
			name:     "another command",
			args:     []string{"okteto", "deploy"},
			expected: false,
		},
		{
			name:     "no command",
			args:     []string{"okteto"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, IsGitCredentialCommand(tt.args))
		})
	}
}
