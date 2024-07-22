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

package externalresource

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func reset(name string, er *ExternalResource) {
	sanitizedExternalName := sanitizeForEnv(name)
	for _, endpoint := range er.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		endpointUrlEnv := fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", sanitizedExternalName, sanitizedEndpointName)
		os.Unsetenv(endpointUrlEnv)
	}
}

func TestExternalResource_SetDefaults(t *testing.T) {
	externalResourceName := "myExternalApp"
	externalResource := ExternalResource{
		Icon: "myIcon",
		Notes: &Notes{
			Path:     "/some/path",
			Markdown: "",
		},
		Endpoints: []*ExternalEndpoint{
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

	defer reset(externalResourceName, &externalResource)
	externalResource.SetDefaults(externalResourceName)

	sanitizedExternalName := sanitizeForEnv(externalResourceName)
	for _, endpoint := range externalResource.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		assert.Equal(t, endpoint.Url, os.Getenv(fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", sanitizedExternalName, sanitizedEndpointName)))
	}
}

func TestExternalResource_LoadMarkdownContent(t *testing.T) {
	manifestPath := "/test/okteto.yml"
	markdownContent := "## Markdown content"
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/test/external/readme.md", []byte(markdownContent), 0600)
	assert.NoError(t, err)

	tests := []struct {
		externalResourceFSM ERFilesystemManager
		name                string
		expectedResult      ExternalResource
		expectErr           bool
	}{
		{
			name: "markdown not found",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Icon: "myIcon",
					Notes: &Notes{
						Path: "/readme.md",
					},
					Endpoints: []*ExternalEndpoint{},
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
					Endpoints: []*ExternalEndpoint{},
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
				Endpoints: []*ExternalEndpoint{},
			},
		},
		{
			name: "notes info not present in external. No markdown loaded",
			externalResourceFSM: ERFilesystemManager{
				ExternalResource: ExternalResource{
					Endpoints: []*ExternalEndpoint{
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
				Endpoints: []*ExternalEndpoint{
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

func TestExternalResource_SetURLUsingEnvironFile(t *testing.T) {
	externalResourceName := "test"
	newURLvalue := "/new/url/value"
	tests := []struct {
		expectedErr              error
		envsToSet                map[string]string
		externalResource         *ExternalResource
		expectedExternalResource *ExternalResource
		name                     string
	}{
		{
			name: "error - empty url value",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
					},
				},
			},
			expectedErr: assert.AnError,
		},
		{
			name: "url value to add",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
					},
				},
			},
			expectedExternalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  newURLvalue,
					},
				},
			},
			envsToSet: map[string]string{
				"OKTETO_EXTERNAL_TEST_ENDPOINTS_ENDPOINT1_URL": newURLvalue,
			},
		},
		{
			name: "no url value to overwrite",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/url/for/endpoint/1",
					},
				},
			},
			expectedExternalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/url/for/endpoint/1",
					},
				},
			},
		},
		{
			name: "url value to overwrite",
			externalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/url/for/endpoint/1",
					},
					{
						Name: "endpoint2",
						Url:  "/url/for/endpoint/2",
					},
				},
			},
			expectedExternalResource: &ExternalResource{
				Endpoints: []*ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  newURLvalue,
					},
					{
						Name: "endpoint2",
						Url:  "/url/for/endpoint/2",
					},
				},
			},
			envsToSet: map[string]string{
				"OKTETO_EXTERNAL_TEST_ENDPOINTS_ENDPOINT1_URL": newURLvalue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.externalResource.SetURLUsingEnvironFile(externalResourceName, tt.envsToSet)
			if tt.expectedErr != nil {
				assert.Error(t, err)
			}

			if err == nil {
				assert.Equal(t, tt.expectedExternalResource, tt.externalResource)
			}
		})
	}
}
func TestSection_IsEmpty(t *testing.T) {
	tests := []struct {
		section  Section
		name     string
		expected bool
	}{
		{
			name:     "empty section",
			section:  Section{},
			expected: true,
		},
		{
			name: "non-empty section",
			section: Section{
				"external1": &ExternalResource{},
				"external2": &ExternalResource{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.section.IsEmpty()
			assert.Equal(t, tt.expected, actual)
		})
	}
}
