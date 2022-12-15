package externalresource

import (
	b64 "encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestExternalResource_SetDefaults(t *testing.T) {
	externalResourceName := "myExternalApp"
	externalResource := ExternalResource{
		Icon: "myIcon",
		Notes: Notes{
			Path:     "/some/path",
			Markdown: "",
		},
		Endpoints: []ExternalEndpoint{
			{
				Name: "endpoint1",
				Url:  "/some/url/endpoint1",
			},
			{
				Name: "endpoint2",
				Url:  "/some/url/endpoint2",
			},
		},
	}

	defer reset(externalResourceName, externalResource)
	externalResource.SetDefaults(externalResourceName)

	for _, endpoint := range externalResource.Endpoints {
		assert.Equal(t, endpoint.Url, os.Getenv(fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", externalResourceName, endpoint.Name)))
	}
}

func reset(erName string, er ExternalResource) {
	for _, endpoint := range er.Endpoints {
		endpointUrlEnv := fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", erName, endpoint.Name)
		os.Unsetenv(endpointUrlEnv)
	}
}

func TestExternalResource_LoadMarkdownContent(t *testing.T) {
	manifestPath := "/test"
	markdownContent := "## Markdown content"
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/test/external/readme.md", []byte(markdownContent), 0600)

	tests := []struct {
		name                string
		externalResourceFSM ERFilesystemManager
		expectErr           bool
	}{
		{
			name: "markdown not found",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Icon: "myIcon",
					Notes: Notes{
						Path: "/readme.md",
					},
					Endpoints: []ExternalEndpoint{},
				},
				Fs: fs,
			},
			expectErr: true,
		},
		{
			name: "valid markdown",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Icon: "myIcon",
					Notes: Notes{
						Path: "/external/readme.md",
					},
					Endpoints: []ExternalEndpoint{},
				},
				Fs: fs,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.externalResourceFSM.LoadMarkdownContent(manifestPath); err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}

			if tt.expectErr {
				t.Fatal("didn't got expected error")
			}

			sDec, err := b64.StdEncoding.DecodeString(tt.externalResourceFSM.ExternalResource.Notes.Markdown)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, string(sDec), markdownContent)
		})
	}
}
