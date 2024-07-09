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

package okteto

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestUseManifestFlag(t *testing.T) {
	type expected struct {
		manifestPath  string
		err           error
		newWorkingDir string
	}

	tests := []struct {
		name     string
		flag     string
		mockFs   func() afero.Fs
		expected expected
	}{
		{
			name: "empty manifest flag path",
			mockFs: func() afero.Fs {
				return afero.NewMemMapFs()
			},
			expected: expected{
				manifestPath: "",
				err:          nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.mockFs()
			manifestPath, err := UseManifestPathFlag(fs, tt.flag)

			assert.Equal(t, tt.expected.manifestPath, manifestPath)

			if tt.expected.manifestPath != "" {
				wd, err := os.Getwd()
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.newWorkingDir, wd)
			}

			if tt.expected.err != nil {
				assert.Error(t, tt.expected.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
