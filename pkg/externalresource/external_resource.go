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

// ExternalResourceSection represents the map of external resources at a manifest
type ExternalResourceSection map[string]*ExternalResource

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
	ExternalResource ExternalResource
	Fs               afero.Fs
}

func sanitizeForEnv(name string) string {
	whithoutSpaces := strings.ReplaceAll(name, " ", "_")
	return strings.ToUpper(strings.ReplaceAll(whithoutSpaces, "-", "_"))
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

func (er *ExternalResource) SetURLUsingEnvironFile(name string) error {
	for _, endpoint := range er.Endpoints {
		urlEnvKey := fmt.Sprintf(urlEnvFormat, sanitizeForEnv(name), sanitizeForEnv(endpoint.Name))
		urlValue := os.Getenv(urlEnvKey)
		if urlValue != "" {
			if endpoint.Url != "" {
				oktetoLog.Warning("the value of the URL '%s' for the external resource '%s' will be overwritten.", endpoint.Name, name)
			}
			endpoint.Url = urlValue
		}

		if endpoint.Url == "" {
			return fmt.Errorf("no value associated to the url '%s' of the external resource '%s'.", endpoint.Name, name)
		}
	}

	return nil
}
