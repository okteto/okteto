package discovery

import (
	"path/filepath"

	"github.com/spf13/afero"
)

func getInferredManifestFilePath(cwd string, fs afero.Fs) string {
	if oktetoManifestPath, err := GetOktetoManifestPathWithFilesystem(cwd, fs); err == nil {
		return oktetoManifestPath
	}
	if pipelinePath, err := GetOktetoPipelinePathWithFilesystem(cwd, fs); err == nil {
		return pipelinePath
	}
	if composePath, err := GetComposePathWithFilesystem(cwd, fs); err == nil {
		return composePath
	}
	if chartPath, err := GetHelmChartPathWithFilesystem(cwd, fs); err == nil {
		return chartPath
	}
	if k8sManifestPath, err := GetK8sManifestPathWithFilesystem(cwd, fs); err == nil {
		return k8sManifestPath
	}
	return ""
}

func FindManifestNameWithFilesystem(cwd string, fs afero.Fs) string {
	path := getInferredManifestFilePath(cwd, fs)
	if path == "" {
		return ""
	}
	name, err := filepath.Rel(cwd, path)
	if err != nil {
		return ""
	}
	return name
}
