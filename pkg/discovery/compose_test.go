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

package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetComposePathWhenExists(t *testing.T) {
	var tests = []struct {
		name          string
		expected      string
		filesToCreate []string
	}{
		{
			name:          "docker-compose file exists on wd",
			filesToCreate: []string{"docker-compose.yml"},
			expected:      "docker-compose.yml",
		},
		{
			name:          "compose file exists on wd",
			filesToCreate: []string{"compose.yml"},
			expected:      "compose.yml",
		},
		{
			name:          "docker-compose file exists on .okteto",
			filesToCreate: []string{filepath.Join(".okteto", "docker-compose.yml")},
			expected:      filepath.Join(".okteto", "docker-compose.yml"),
		},
		{
			name:          "compose file exists on .okteto",
			filesToCreate: []string{filepath.Join(".okteto", "compose.yml")},
			expected:      filepath.Join(".okteto", "compose.yml"),
		},
		{
			name:          "okteto-stack file exists on wd",
			filesToCreate: []string{"okteto-stack.yml"},
			expected:      "okteto-stack.yml",
		},
		{
			name:          "okteto-stack file exists on .okteto",
			filesToCreate: []string{filepath.Join(".okteto", "okteto-stack.yml")},
			expected:      filepath.Join(".okteto", "okteto-stack.yml"),
		},
		{
			name:          "docker-compose and okteto stack files exists on wd",
			filesToCreate: []string{"docker-compose.yml", "okteto-stack.yml"},
			expected:      "okteto-stack.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			for _, fileToCreate := range tt.filesToCreate {
				fullpath := filepath.Join(wd, fileToCreate)
				assert.NoError(t, os.MkdirAll(filepath.Dir(fullpath), 0750))
				f, err := os.Create(fullpath)
				assert.NoError(t, err)
				defer func() {
					if err := f.Close(); err != nil {
						t.Fatalf("Error closing file %s: %s", fullpath, err)
					}
				}()
			}
			result, err := GetComposePath(wd)
			assert.NoError(t, err)
			assert.Equal(t, filepath.Join(wd, tt.expected), result)
		})
	}
}

func TestGetComposePathWhenNotExists(t *testing.T) {
	wd := t.TempDir()
	result, err := GetComposePath(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, ErrComposeFileNotFound)
}
