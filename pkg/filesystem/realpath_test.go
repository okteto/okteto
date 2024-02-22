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

package filesystem

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealpath(t *testing.T) {
	fs := afero.NewMemMapFs()
	testFilePath, err := filepath.Abs("/path/to/TestFile")
	require.NoError(t, err)
	err = fs.MkdirAll(testFilePath, 0755)
	require.NoError(t, err)

	tt := []struct {
		expectedErr error
		name        string
		path        string
		expected    string
	}{
		{
			name:        "case insensitive match",
			path:        filepath.Clean("/patH/to/testfile"),
			expected:    filepath.Clean("/path/to/TestFile"),
			expectedErr: nil,
		},
		{
			name:        "exact match",
			path:        filepath.Clean("/path/to/TestFile"),
			expected:    filepath.Clean("/path/to/TestFile"),
			expectedErr: nil,
		},
		{
			name:        "not found",
			path:        filepath.Clean("/path/to/NotFound"),
			expected:    "",
			expectedErr: errRealPathNotFound,
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			absInputPath, err := filepath.Abs(test.path)
			require.NoError(t, err)

			var absExpectedPath string
			if test.expected != "" {
				absExpectedPath, err = filepath.Abs(test.expected)
				require.NoError(t, err)
			}

			realPath, err := Realpath(fs, absInputPath)
			require.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, absExpectedPath, realPath)
		})
	}
}
