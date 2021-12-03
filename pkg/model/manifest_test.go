package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetBuildManifest(t *testing.T) {
	tests := []struct {
		name              string
		manifestYAML      []byte
		manifestNotExists bool
		expectedErr       bool
		expectedManifest  *ManifestBuild
	}{
		{
			name: "exists-manifest-yaml",
			manifestYAML: []byte(`build:
  firstImage: ./first
  secondImage:
    context: ./second`),
			expectedErr: false,
			expectedManifest: &ManifestBuild{
				"firstImage":  {Context: "./first"},
				"secondImage": {Context: "./second"},
			},
		},
		{
			name: "exists-manifest-yml",
			manifestYAML: []byte(`build:
  firstImage: ./first
  secondImage:
    context: ./second`),
			expectedErr: false,
			expectedManifest: &ManifestBuild{
				"firstImage":  {Context: "./first"},
				"secondImage": {Context: "./second"},
			},
		},
		{
			name:              "not-exists-manifest",
			expectedErr:       true,
			manifestNotExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tmpFile, err := os.CreateTemp("", "")
			if err != nil {
				t.Fatalf("failed to create dynamic manifest file: %s", err.Error())
			}
			if err := os.WriteFile(tmpFile.Name(), []byte(tt.manifestYAML), 0600); err != nil {
				t.Fatalf("failed to write manifest file: %s", err.Error())
			}
			defer os.RemoveAll(tmpFile.Name())

			if tt.manifestNotExists {
				os.RemoveAll(tmpFile.Name())
			}

			m, err := GetBuildManifest(tmpFile.Name())
			if tt.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.EqualValues(t, tt.expectedManifest, m)
			}

		})
	}
}
