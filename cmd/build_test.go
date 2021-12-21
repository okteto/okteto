package cmd

import (
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_isManifestV2(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		fileName     string
		manifestYAML []byte
		result       struct {
			manifest model.ManifestBuild
			ok       bool
		}
	}{
		{
			name: "env-is-not-enabled",
			env:  "false",
			result: struct {
				manifest model.ManifestBuild
				ok       bool
			}{
				manifest: nil,
				ok:       false,
			},
		},
		{
			name: "env-is-not-valid",
			env:  "not-valid-value",
			result: struct {
				manifest model.ManifestBuild
				ok       bool
			}{
				manifest: nil,
				ok:       false,
			},
		},
		{
			name: "env-is-enabled-file-not-found",
			env:  "true",
			result: struct {
				manifest model.ManifestBuild
				ok       bool
			}{
				manifest: nil,
				ok:       false,
			},
		},
		{
			name:     "env-is-enabled-file-empty",
			env:      "true",
			fileName: "okteto.yaml",
			result: struct {
				manifest model.ManifestBuild
				ok       bool
			}{
				manifest: nil,
				ok:       false,
			},
		},
		{
			name:     "env-is-enabled-file-exists",
			env:      "true",
			fileName: "okteto.yaml",
			manifestYAML: []byte(`
build:
  service:
    context: .
    dockerfile: Dockerfile`),
			result: struct {
				manifest model.ManifestBuild
				ok       bool
			}{
				manifest: model.ManifestBuild{
					"service": {
						Context:    ".",
						Dockerfile: "Dockerfile",
					},
				},
				ok: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OKTETO_ENABLE_MANIFEST_V2", tt.env)

			filename := ""
			if tt.fileName != "" {
				file, err := os.CreateTemp("", "test")
				if err != nil {
					t.Error(err)
				}
				defer os.RemoveAll(file.Name())

				filename = file.Name()
				if err := os.WriteFile(filename, tt.manifestYAML, 0600); err != nil {
					t.Log(err)
				}
			}

			rm, rok := isManifestV2(filename)

			assert.Equal(t, tt.result.manifest, rm)
			assert.Equal(t, tt.result.ok, rok)
			os.Unsetenv("OKTETO_ENABLE_MANIFEST_V2")
		})
	}
}
