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

func TestGetContextResourcePathWhenExists(t *testing.T) {
	var tests = []struct {
		expectErr     error
		name          string
		expected      string
		filesToCreate []string
	}{
		{
			name:          "manifest does not exists",
			filesToCreate: []string{},
			expected:      "",
			expectErr:     ErrOktetoManifestNotFound,
		},
		{
			name:          "docker-compose file exists",
			filesToCreate: []string{"docker-compose.yml"},
			expected:      "docker-compose.yml",
		},
		{
			name:          "okteto file exists",
			filesToCreate: []string{"okteto.yml"},
			expected:      "okteto.yml",
		},
		{
			name:          "okteto pipeline file exists",
			filesToCreate: []string{"okteto-pipeline.yml"},
			expected:      "okteto-pipeline.yml",
		},
		{
			name:          "okteto pipeline and okteto manifest exists",
			filesToCreate: []string{"okteto-pipeline.yml", "okteto.yml"},
			expected:      "okteto.yml",
		},
		{
			name:          "okteto pipeline and compose file exists",
			filesToCreate: []string{"docker-compose.yml", "okteto-pipeline.yml"},
			expected:      "okteto-pipeline.yml",
		},
		{
			name:          "okteto manifest and compose file exists",
			filesToCreate: []string{"docker-compose.yml", "okteto.yml"},
			expected:      "okteto.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			for _, fileToCreate := range tt.filesToCreate {
				fullpath := filepath.Join(wd, fileToCreate)
				f, err := os.Create(fullpath)
				assert.NoError(t, err)
				defer func() {
					if err := f.Close(); err != nil {
						t.Fatalf("Error closing file %s: %s", fullpath, err)
					}
				}()
			}
			result, err := GetContextResourcePath(wd)
			assert.ErrorIs(t, tt.expectErr, err)
			if result != "" {
				result = filepath.Base(result)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetContextResourcePathWhenNotExists(t *testing.T) {
	wd := t.TempDir()
	result, err := GetContextResourcePath(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, ErrOktetoManifestNotFound)
}
