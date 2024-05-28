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

package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_UpdateCWDtoManifestPath(t *testing.T) {
	root := t.TempDir()
	var tests = []struct {
		name         string
		path         string
		expectedPath string
		expectedErr  bool
	}{
		{
			name:         "inside .okteto folder",
			path:         filepath.Join(root, ".okteto", "okteto.yml"),
			expectedPath: filepath.Join(".okteto", "okteto.yml"),
		},
		{
			name:         "one path ahead",
			path:         filepath.Join(root, "test", "okteto.yml"),
			expectedPath: "okteto.yml",
		},
		{
			name:         "same path",
			path:         filepath.Join(root, "okteto.yml"),
			expectedPath: "okteto.yml",
		},
		{
			name:         "full path on .okteto",
			path:         filepath.Join(root, "usr", ".okteto", "okteto.yml"),
			expectedPath: filepath.Join(".okteto", "okteto.yml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			initialCWD, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			if err := os.Chdir(root); err != nil {
				t.Fatal(err)
			}

			if err := os.MkdirAll(tt.path, 0700); err != nil {
				t.Fatal(err)
			}

			res, err := UpdateCWDtoManifestPath(tt.path)
			if tt.expectedErr && err == nil {
				t.Fatal("expected err")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("not expected error, got %v", err)
			}

			assert.Equal(t, tt.expectedPath, res)

			if err := os.Chdir(initialCWD); err != nil {
				t.Fatal(err)
			}
		})
	}
}
