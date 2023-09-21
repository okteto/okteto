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
		config          config
		expectedContent string
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
