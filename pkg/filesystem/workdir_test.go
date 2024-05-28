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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetWorkdirFromManifest(t *testing.T) {
	var tests = []struct {
		name         string
		path         string
		expectedPath string
	}{
		{
			name:         "inside .okteto folder",
			path:         filepath.Join(".okteto", "okteto.yml"),
			expectedPath: ".",
		},
		{
			name:         "one path ahead",
			path:         filepath.Join("test", "okteto.yml"),
			expectedPath: "test",
		},
		{
			name:         "same path",
			path:         "okteto.yml",
			expectedPath: ".",
		},
		{
			name:         "full path",
			path:         filepath.Join("/usr", "okteto.yml"),
			expectedPath: filepath.Clean("/usr"),
		},
		{
			name:         "full path on .okteto",
			path:         filepath.Clean("/usr/.okteto/okteto.yml"),
			expectedPath: filepath.Clean("/usr"),
		},
		{
			name:         "relative path with more than two paths ahead",
			path:         filepath.Join("~", "app", ".okteto", "okteto.yml"),
			expectedPath: filepath.Join("~", "app"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetWorkdirFromManifestPath(tt.path)
			assert.Equal(t, tt.expectedPath, result)
			newManifestPath := GetManifestPathFromWorkdir(tt.path, result)
			if strings.Contains(tt.path, ".okteto") {
				assert.Equal(t, filepath.Join(".okteto", "okteto.yml"), newManifestPath)
			} else {
				assert.Equal(t, "okteto.yml", newManifestPath)
			}

		})
	}
}

func Test_GetManifestPathFromWorkdir(t *testing.T) {
	var tests = []struct {
		name         string
		path         string
		workdir      string
		expectedPath string
	}{
		{
			name:         "inside .okteto folder",
			path:         filepath.Join(".okteto", "okteto.yml"),
			workdir:      ".",
			expectedPath: filepath.Join(".okteto", "okteto.yml"),
		},
		{
			name:         "one path ahead",
			path:         filepath.Join("test", "okteto.yml"),
			workdir:      "test",
			expectedPath: "okteto.yml",
		},
		{
			name:         "same path",
			path:         "okteto.yml",
			workdir:      ".",
			expectedPath: "okteto.yml",
		},
		{
			name:         "full path",
			path:         filepath.Join("/usr", "okteto.yml"),
			workdir:      "/usr",
			expectedPath: "okteto.yml",
		},
		{
			name:         "full path on .okteto",
			path:         filepath.Clean("/usr/.okteto/okteto.yml"),
			workdir:      "/usr",
			expectedPath: filepath.Join(".okteto", "okteto.yml"),
		},
		{
			name:         "relative path with more than two paths ahead",
			path:         filepath.Join("~", "app", ".okteto", "okteto.yml"),
			workdir:      "~/app",
			expectedPath: filepath.Join(".okteto", "okteto.yml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GetManifestPathFromWorkdir(tt.path, tt.workdir)
			assert.Equal(t, tt.expectedPath, res)
		})
	}
}

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
