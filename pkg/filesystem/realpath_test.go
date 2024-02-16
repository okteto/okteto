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

package filesystem

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealpath(t *testing.T) {
	fs := afero.NewMemMapFs()
	testFilePath := "/path/to/TestFile"
	err := fs.MkdirAll(testFilePath, 0755)
	require.NoError(t, err)

	tt := []struct {
		name        string
		path        string
		expected    string
		expectedErr error
	}{
		{
			name:        "case insensitive match",
			path:        "/patH/to/testfile",
			expected:    "/path/to/TestFile",
			expectedErr: nil,
		},
		{
			name:        "exact match",
			path:        "/path/to/TestFile",
			expected:    "/path/to/TestFile",
			expectedErr: nil,
		},
		{
			name:        "not found",
			path:        "/path/to/NotFound",
			expected:    "",
			expectedErr: errRealPathNotFound,
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			realPath, err := Realpath(fs, test.path)
			require.ErrorIs(t, err, test.expectedErr)
			assert.Equal(t, test.expected, realPath)
		})
	}
}
