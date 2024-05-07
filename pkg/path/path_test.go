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

package path

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetRelativePathFromCWD(t *testing.T) {
	root := t.TempDir()
	var tests = []struct {
		name         string
		path         string
		cwd          string
		expectedPath string
		expectedErr  bool
	}{
		{
			name:         "inside .okteto folder",
			path:         filepath.Join(root, ".okteto", "okteto.yml"),
			cwd:          root,
			expectedPath: filepath.Join(".okteto", "okteto.yml"),
		},
		{
			name:         "one path ahead - cwd is folder",
			path:         filepath.Join(root, "test", "okteto.yml"),
			cwd:          filepath.Join(root, "test"),
			expectedPath: "okteto.yml",
		},
		{
			name:         "one path ahead - cwd is root",
			path:         filepath.Join(root, "test", "okteto.yml"),
			cwd:          root,
			expectedPath: filepath.Join("test", "okteto.yml"),
		},
		{
			name:         "one path ahead not abs - cwd is root",
			path:         filepath.Join("test", "okteto.yml"),
			cwd:          root,
			expectedPath: filepath.Join("test", "okteto.yml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := GetRelativePathFromCWD(tt.cwd, tt.path)
			if tt.expectedErr && err == nil {
				t.Fatal("expected err")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("not expected error, got %v", err)
			}

			assert.Equal(t, tt.expectedPath, res)

		})
	}
}
