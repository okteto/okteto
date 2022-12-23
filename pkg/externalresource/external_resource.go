package externalresource

import (
	b64 "encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// ExternalResourceSection represents the map of external resources at a manifest
type ExternalResourceSection map[string]*ExternalResource

// ExternalResource represents information on an external resource
type ExternalResource struct {
	Icon      string
	Notes     *Notes
	Endpoints []ExternalEndpoint
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
	ExternalResource ExternalResource
	Fs               afero.Fs
}

// SetDefaults creates the necessary environment variables given an external resource
func (er *ExternalResource) SetDefaults(externalName string) {
	sanitizedExternalName := sanitizeForEnv(externalName)
	for _, endpoint := range er.Endpoints {
		sanitizedEndpointName := sanitizeForEnv(endpoint.Name)
		endpointUrlEnv := fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", sanitizedExternalName, sanitizedEndpointName)
		os.Setenv(endpointUrlEnv, endpoint.Url)
	}
}

func sanitizeForEnv(name string) string {
	whithoutSpaces := strings.ReplaceAll(name, " ", "_")
	return strings.ToUpper(strings.ReplaceAll(whithoutSpaces, "-", "_"))
}

// LoadMarkdownContent loads and store markdown content related to external resource
func (ef *ERFilesystemManager) LoadMarkdownContent(manifestPath string) error {
	markdownAbsPath := filepath.Join(manifestPath, ef.ExternalResource.Notes.Path)
	b, err := afero.ReadFile(ef.Fs, markdownAbsPath)
	if err != nil {
		return err
	}

	ef.ExternalResource.Notes.Markdown = b64.StdEncoding.EncodeToString([]byte(string(b)))
	return nil
}
