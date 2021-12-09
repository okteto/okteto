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
		expectedManifest  ManifestBuild
	}{
		{
			name:              "not-exists-manifest",
			expectedErr:       true,
			manifestNotExists: true,
		},
		{
			name: "relative-path-build",
			manifestYAML: []byte(`build:
  service: ./service`),
			expectedErr: false,
			expectedManifest: ManifestBuild{
				"service": {
					Name:       "",
					Target:     "",
					Context:    "./service",
					Dockerfile: "Dockerfile",
					Image:      "",
					Args:       nil,
					CacheFrom:  nil,
				},
			},
		},
		{
			name: "all-defined-fields",
			manifestYAML: []byte(`build:
  service:
    image: defined-tag-image
    context: ./service
    target: build
    dockerfile: custom-dockerfile
    args:
      KEY1: Value1
      KEY2: Value2
    secrets:
      - KEY1=Value1
      - KEY2=Value2
    cache_from:
      - cache-image-1
      - cache-image-2`),
			expectedErr: false,
			expectedManifest: ManifestBuild{
				"service": {
					Name:       "",
					Target:     "build",
					Context:    "./service",
					Dockerfile: "custom-dockerfile",
					Image:      "defined-tag-image",
					Args: []EnvVar{
						{
							Name: "KEY1", Value: "Value1",
						},
						{
							Name: "KEY2", Value: "Value2",
						},
					},
					Secrets:   []string{"KEY1=Value1", "KEY2=Value2"},
					CacheFrom: []string{"cache-image-1", "cache-image-2"},
				},
			},
		},
		{
			name: "default-values",
			manifestYAML: []byte(`build:
  service:
    args:
      KEY1: Value1
      KEY2: Value2
    secrets:
      - KEY1=Value1
      - KEY2=Value2
    cache_from:
      - cache-image-1
      - cache-image-2`),
			expectedErr: false,
			expectedManifest: ManifestBuild{
				"service": {
					Name:       "",
					Target:     "",
					Context:    "./service",
					Dockerfile: "Dockerfile",
					Image:      "",
					Args: []EnvVar{
						{
							Name: "KEY1", Value: "Value1",
						},
						{
							Name: "KEY2", Value: "Value2",
						},
					},
					Secrets:   []string{"KEY1=Value1", "KEY2=Value2"},
					CacheFrom: []string{"cache-image-1", "cache-image-2"},
				},
			},
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
				assert.Equal(t, tt.expectedManifest, m)
			}

		})
	}
}
