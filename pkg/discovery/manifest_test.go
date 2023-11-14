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

func TestGetOktetoManifestPathWhenExists(t *testing.T) {
	var tests = []struct {
		name          string
		expected      string
		filesToCreate []string
	}{
		{
			name:          "okteto manifest file exists on wd",
			filesToCreate: []string{"okteto.yml"},
			expected:      "okteto.yml",
		},
		{
			name:          "okteto manifest file exists on .okteto",
			filesToCreate: []string{filepath.Join(".okteto", "okteto.yml")},
			expected:      filepath.Join(".okteto", "okteto.yml"),
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
			result, err := GetOktetoManifestPath(wd)
			assert.NoError(t, err)
			assert.Equal(t, filepath.Join(wd, tt.expected), result)
		})
	}
}

func TestGetOktetoPathWhenNotExists(t *testing.T) {
	wd := t.TempDir()
	result, err := GetOktetoManifestPath(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, ErrOktetoManifestNotFound)
}
