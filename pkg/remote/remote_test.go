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

package remote

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestCreateDockerignoreFileWithFilesystem(t *testing.T) {
	dockerignoreWd := "/test/"

	type config struct {
		wd               string
		manifestPathFlag string
		hasManifest      bool
	}
	var tests = []struct {
		name            string
		expectedContent string
		config          config
	}{
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore without manifest",
			config: config{
				wd: dockerignoreWd,
			},
			expectedContent: "FROM alpine\n",
		},
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore with manifest",
			config: config{
				wd:          dockerignoreWd,
				hasManifest: true,
			},
			expectedContent: "FROM alpine\n!okteto.yaml\n",
		},
		{
			name: "dockerignore present copy .oktetodeployignore to .dockerignore with manifestPath not empty",
			config: config{
				wd:               dockerignoreWd,
				manifestPathFlag: "file-flag-okteto.yaml",
			},
			expectedContent: "FROM alpine\n!file-flag-okteto.yaml\n",
		},
		{
			name: "without dockerignore, with manifest file flag, generate with ignore file content",
			config: config{
				manifestPathFlag: "file-flag-okteto.yaml",
			},
			expectedContent: "!file-flag-okteto.yaml\n",
		},
		{
			name: "without dockerignore, with manifest, generate with ignore manifest content",
			config: config{
				hasManifest: true,
			},
			expectedContent: "!okteto.yaml\n",
		},
		{
			name:            "without dockerignore, without manifest generate empty dockerignore",
			config:          config{},
			expectedContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			fs := afero.NewMemMapFs()

			assert.NoError(t, fs.MkdirAll(dockerignoreWd, 0755))
			assert.NoError(t, afero.WriteFile(fs, filepath.Join(dockerignoreWd, ".oktetodeployignore"), []byte("FROM alpine"), 0644))
			if tt.config.hasManifest {
				assert.NoError(t, afero.WriteFile(fs, filepath.Join(tt.config.wd, "okteto.yaml"), []byte("hola"), 0644))
			}

			err := CreateDockerignoreFileWithFilesystem(tt.config.wd, tempDir, tt.config.manifestPathFlag, fs)
			assert.NoError(t, err)
			b, err := afero.ReadFile(fs, filepath.Join(tempDir, ".dockerignore"))
			assert.Equal(t, tt.expectedContent, string(b))
			assert.NoError(t, err)

		})
	}
}
