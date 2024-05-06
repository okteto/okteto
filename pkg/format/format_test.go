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

package format

import "testing"

func Test_SanitizeName(t *testing.T) {
	var tests = []struct {
		name     string
		expected string
	}{
		{
			name:     "okteto",
			expected: "okteto",
		},
		{
			name:     "storyBooks.git",
			expected: "storybooks-git",
		},
		{
			name:     "kubernetes-bitwarden_rs",
			expected: "kubernetes-bitwarden-rs",
		},
		{
			name:     "my repository is /okteto (name) ",
			expected: "my-repository-is-okteto-name",
		},
		{
			name:     "this    has more spaces  and //okteto (name)",
			expected: "this-has-more-spaces-and-okteto-name",
		},
		{
			name:     "(this)    has more spaces  and //okteto (name)",
			expected: "this-has-more-spaces-and-okteto-name",
		},
		{
			name:     "a very long name for the repository a very long name for the re THIS IS REMOVED",
			expected: "a-very-long-name-for-the-repository-a-very-long-name-for-the-re",
		},
		{
			name:     "a text with _ and (this)_",
			expected: "a-text-with-and-this",
		},
		{
			name:     "a very long name for the repository a very long name for the r_ THIS IS REMOVED WITH NO HYPHEN",
			expected: "a-very-long-name-for-the-repository-a-very-long-name-for-the-r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResourceK8sMetaString(tt.name)
			if got != tt.expected {
				t.Errorf("got %s, expected %s", got, tt.expected)
			}
		})
	}
}
