package manifest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"gopkg.in/yaml.v3"
)

// RunCreateTemplate generates a manifest template based on provided options.
func (*Command) RunCreateTemplate(ctx context.Context, opts *InitOpts) (*model.Manifest, error) {
	path, err := determinePath(opts)
	if err != nil {
		return nil, err
	}

	if err := validateDevPath(path, opts.Overwrite); err != nil {
		return nil, err
	}

	tmplCnt, err := getUriContents(opts.Template)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("okteto manifest").Parse(tmplCnt)
	if err != nil {
		return nil, err
	}

	targs, err := prepareTemplateArgs(opts)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, targs)
	if err != nil {
		return nil, err
	}

	oktetoLog.Success("Generated template successfully.")
	return model.Read(buffer.Bytes())
}

// determinePath sets the path for the manifest.
func determinePath(opts *InitOpts) (string, error) {
	if opts.DevPath != "" {
		return opts.DevPath, nil
	}
	return filepath.Join(opts.Workdir, utils.DefaultManifest), nil
}

// getUriContents fetches the template content from a URI or file path.
func getUriContents(tmplUri string) (string, error) {
	if isHttpUrl(tmplUri) {
		return fetchUrlContent(tmplUri)
	}
	return fetchFileContent(tmplUri)
}

// fetchUrlContent retrieves content from a URL.
func fetchUrlContent(tmplUri string) (string, error) {
	resp, err := http.Get(tmplUri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: %s, status code: %d", tmplUri, resp.StatusCode)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// fetchFileContent retrieves content from a file.
func fetchFileContent(filePath string) (string, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// isHttpUrl checks if a string is a valid HTTP/HTTPS URL.
func isHttpUrl(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

// prepareTemplateArgs prepares the template arguments.
func prepareTemplateArgs(opts *InitOpts) (map[string]interface{}, error) {
	ns := opts.Namespace
	ctxName := opts.Context
	if ns == "" {
		ns = okteto.GetContext().Namespace
	}
	if ctxName == "" {
		ctxName = okteto.GetContext().Name
	}

	if opts.TemplateArgs == nil {
		opts.TemplateArgs = make(map[string]string)
	}
	opts.TemplateArgs["namespace"] = ns
	opts.TemplateArgs["context"] = ctxName

	targs := make(map[string]interface{})
	if opts.TemplateArgFile != "" {
		loadedArgs, err := loadArgumentFiles(opts.TemplateArgFile)
		if err != nil {
			return nil, err
		}
		for k, v := range loadedArgs {
			targs[k] = v
		}
	}

	// load inline arguments
	for k, v := range opts.TemplateArgs {
		targs[k] = v
	}

	return targs, nil
}

// loadArgumentFiles loads template arguments from a YAML file.
func loadArgumentFiles(path string) (map[string]interface{}, error) {
	argsMap := make(map[string]interface{})
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(file, &argsMap)
	if err != nil {
		return nil, err
	}
	return argsMap, nil
}
