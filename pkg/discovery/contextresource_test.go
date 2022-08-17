// Copyright 2022 The Okteto Authors
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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetContextResourcePathWhenExists(t *testing.T) {
	var tests = []struct {
		name          string
		filesToCreate []string
		expected      string
		expectErr     error
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
				defer f.Close()
			}
			result, err := GetContextResourcePath(wd)
			assert.ErrorIs(t, tt.expectErr, err)

			splittedPath := strings.Split(result, "/")
			assert.Equal(t, tt.expected, splittedPath[len(splittedPath)-1])
		})
	}
}

func TestGetContextResourcePathWhenNotExists(t *testing.T) {
	wd := t.TempDir()
	result, err := GetContextResourcePath(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, ErrOktetoManifestNotFound)
}
