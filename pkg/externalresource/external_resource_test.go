package externalresource

import (
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
		Notes: &Notes{
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

	sanitizedExternalName := sanitizeForEnv(externalResourceName)
	for _, endpoint := range externalResource.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		assert.Equal(t, endpoint.Url, os.Getenv(fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", sanitizedExternalName, sanitizedEndpointName)))
	}
}

func reset(sanitizedExternalName string, er ExternalResource) {
	for _, endpoint := range er.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		endpointUrlEnv := fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", sanitizedExternalName, sanitizedEndpointName)
		os.Unsetenv(endpointUrlEnv)
	}
}

func TestExternalResource_LoadMarkdownContent(t *testing.T) {
	manifestPath := "/test/okteto.yml"
	markdownContent := "## Markdown content"
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/test/external/readme.md", []byte(markdownContent), 0600)

	tests := []struct {
		name                string
		externalResourceFSM ERFilesystemManager
		expectErr           bool
		expectedResult      ExternalResource
	}{
		{
			name: "markdown not found",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Icon: "myIcon",
					Notes: &Notes{
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
					Notes: &Notes{
						Path: "/external/readme.md",
					},
					Endpoints: []ExternalEndpoint{},
				},
				Fs: fs,
			},
			expectErr: false,
			expectedResult: ExternalResource{
				Icon: "myIcon",
				Notes: &Notes{
					Path:     "/external/readme.md",
					Markdown: "IyMgTWFya2Rvd24gY29udGVudA==",
				},
				Endpoints: []ExternalEndpoint{},
			},
		},
		{
			name: "notes info not present in external. No markdown loaded",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Endpoints: []ExternalEndpoint{
						{
							Name: "name1",
							Url:  "/some/url",
						},
					},
				},
				Fs: fs,
			},
			expectErr: false,
			expectedResult: ExternalResource{
				Endpoints: []ExternalEndpoint{
					{
						Name: "name1",
						Url:  "/some/url",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectErr {
				assert.Error(t, tt.externalResourceFSM.LoadMarkdownContent(manifestPath))
			} else {
				assert.NoError(t, tt.externalResourceFSM.LoadMarkdownContent(manifestPath))
				assert.Equal(t, tt.externalResourceFSM.ExternalResource, tt.expectedResult)
			}
		})
	}
}

func TestExternalResource_SanitizeForEnv(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "name with '-'",
			input:          "TEST-NAME",
			expectedOutput: "TEST_NAME",
		},
		{
			name:           "name in lowercase",
			input:          "test",
			expectedOutput: "TEST",
		},
		{
			name:           "name with spaces",
			input:          "test one",
			expectedOutput: "TEST_ONE",
		},
		{
			name:           "name in lowercase, with spaces and with '-'",
			input:          "test-name one",
			expectedOutput: "TEST_NAME_ONE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedOutput, sanitizeForEnv(tt.input))
		})
	}
}
