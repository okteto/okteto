package externalresource

import (
	b64 "encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// ExternalResources represents the map of external resources at a manifest
type ExternalResources map[string]*ExternalResource

// ExternalResources represents information on an external resource
type ExternalResource struct {
	Icon      string
	Notes     Notes
	Endpoints []ExternalEndpoint
}

type Notes struct {
	Path     string
	Markdown string // base64 encoded content of the path
}

type ExternalEndpoint struct {
	Name string
	Url  string
}

type ERFilesystemManager struct {
	ExternalResource ExternalResource
	Fs               afero.Fs
}

func (er *ExternalResource) SetDefaults(externalName string) {
	for _, endpoint := range er.Endpoints {
		endpointUrlEnv := fmt.Sprintf("OKTETO_EXTERNAL_%s_ENDPOINTS_%s_URL", externalName, endpoint.Name)
		os.Setenv(endpointUrlEnv, endpoint.Url)
	}

	return
}

func (ef *ERFilesystemManager) LoadMarkdownContent(manifestPath string) error {
	markdownAbsPath := filepath.Join(manifestPath, ef.ExternalResource.Notes.Path)
	b, err := afero.ReadFile(ef.Fs, markdownAbsPath)
	if err != nil {
		return err
	}

	ef.ExternalResource.Notes.Markdown = b64.StdEncoding.EncodeToString([]byte(string(b)))
	return nil
}
