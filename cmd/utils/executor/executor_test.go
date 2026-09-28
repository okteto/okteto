// Copyright 2025 The Okteto Authors
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

package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_withAliases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		env      []string
		expected []string
	}{
		{
			name:     "nil",
			env:      nil,
			expected: nil,
		},
		{
			name:     "no aliases",
			env:      []string{"MY_VAR=value"},
			expected: []string{"MY_VAR=value"},
		},
		{
			name:     "deprecated name adds new name",
			env:      []string{"OKTETO_ALPHA_BUILD_COMPRESSION=zstd"},
			expected: []string{"OKTETO_ALPHA_BUILD_COMPRESSION=zstd", "OKTETO_BUILD_COMPRESSION=zstd"},
		},
		{
			name:     "new name adds deprecated name",
			env:      []string{"OKTETO_BUILD_COMPRESSION_LEVEL=3"},
			expected: []string{"OKTETO_BUILD_COMPRESSION_LEVEL=3", "OKTETO_ALPHA_BUILD_COMPRESSION_LEVEL=3"},
		},
		{
			name:     "both names are kept as they are",
			env:      []string{"OKTETO_BUILD_FORCE_COMPRESSION=true", "OKTETO_ALPHA_BUILD_FORCE_COMPRESSION=false"},
			expected: []string{"OKTETO_BUILD_FORCE_COMPRESSION=true", "OKTETO_ALPHA_BUILD_FORCE_COMPRESSION=false"},
		},
		{
			name:     "empty value",
			env:      []string{"OKTETO_ALPHA_BUILD_COMPRESSION="},
			expected: []string{"OKTETO_ALPHA_BUILD_COMPRESSION=", "OKTETO_BUILD_COMPRESSION="},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, withAliases(tt.env))
		})
	}
}
