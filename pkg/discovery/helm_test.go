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

func TestGetHelmPathWhenExists(t *testing.T) {
	var tests = []struct {
		name          string
		expected      string
		filesToCreate []string
	}{
		{
			name:          "chart folder exists on wd",
			filesToCreate: []string{filepath.Join("charts", "Chart.yaml")},
			expected:      "charts",
		},
		{
			name:          "chart folder inside helm folder exists on wd",
			filesToCreate: []string{filepath.Join("helm", "charts", "Chart.yaml")},
			expected:      filepath.Join("helm", "charts"),
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
			result, err := GetHelmChartPath(wd)
			assert.NoError(t, err)
			assert.Equal(t, filepath.Join(wd, tt.expected), result)
		})
	}
}

func TestGetHelmPathWhenNotExists(t *testing.T) {
	wd := t.TempDir()
	result, err := GetHelmChartPath(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, ErrHelmChartNotFound)
}
