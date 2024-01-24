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
	b64 "encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

const (
	urlEnvFormat = "OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL"
)

// Section represents the map of external resources at a manifest
type Section map[string]*ExternalResource

// ExternalResource represents information on an external resource
type ExternalResource struct {
	Icon      string
	Notes     *Notes
	Endpoints []*ExternalEndpoint
}

// Notes represents information about the location and content of the external resource markdown
type Notes struct {
	Path     string
	Markdown string // base64 encoded content of the path
}

// ExternalEndpoint represents information about an endpoint
type ExternalEndpoint struct {
	Name string
	Url  string
}

// ERFilesystemManager represents ExternalResource information with the filesystem injected
type ERFilesystemManager struct {
	Fs               afero.Fs
	ExternalResource ExternalResource
}

func sanitizeForEnv(name string) string {
	whithoutSpaces := strings.ReplaceAll(name, " ", "_")
	return strings.ToUpper(strings.ReplaceAll(whithoutSpaces, "-", "_"))
}

// SetDefaults creates the necessary environment variables given an external resource
func (er *ExternalResource) SetDefaults(externalName string) {
	sanitizedExternalName := sanitizeForEnv(externalName)
	for _, endpoint := range er.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		endpointUrlEnv := fmt.Sprintf(urlEnvFormat, sanitizedExternalName, sanitizedEndpointName)
		os.Setenv(endpointUrlEnv, endpoint.Url)
	}
}

// LoadMarkdownContent loads and store markdown content related to external resource
func (ef *ERFilesystemManager) LoadMarkdownContent(manifestPath string) error {

	if ef.ExternalResource.Notes == nil {
		return nil
	}

	markdownAbsPath := filepath.Join(filepath.Dir(manifestPath), ef.ExternalResource.Notes.Path)
	b, err := afero.ReadFile(ef.Fs, markdownAbsPath)
	if err != nil {
		return err
	}

	ef.ExternalResource.Notes.Markdown = b64.StdEncoding.EncodeToString([]byte(string(b)))
	return nil
}

func (er *ExternalResource) SetURLUsingEnvironFile(name string, dynamicEnvs map[string]string) error {
	for _, endpoint := range er.Endpoints {
		urlEnvKey := fmt.Sprintf(urlEnvFormat, sanitizeForEnv(name), sanitizeForEnv(endpoint.Name))
		urlValue := dynamicEnvs[urlEnvKey]
		if urlValue != "" {
			if endpoint.Url != "" {
				oktetoLog.Warning(`the original value of the URL belonging to the endpoint '%s' of the `+
					`external resource '%s' will be overwritten by the one declared in the deploy section.`, endpoint.Name, name)
			}
			endpoint.Url = urlValue
		}

		if endpoint.Url == "" {
			return fmt.Errorf("no value associated to the url '%s' of the external resource '%s'.", endpoint.Name, name)
		}
	}

	return nil
}
