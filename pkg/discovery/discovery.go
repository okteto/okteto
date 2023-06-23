package discovery

import "path/filepath"

func getInferredManifestFilePath(cwd string) string {
	if oktetoManifestPath, err := GetOktetoManifestPath(cwd); err == nil {
		return oktetoManifestPath
	}
	if pipelinePath, err := GetOktetoPipelinePath(cwd); err == nil {
		return pipelinePath
	}
	if composePath, err := GetComposePath(cwd); err == nil {
		return composePath
	}
	if chartPath, err := GetHelmChartPath(cwd); err == nil {
		return chartPath
	}
	if k8sManifestPath, err := GetK8sManifestPath(cwd); err == nil {
		return k8sManifestPath
	}
	return ""
}

func FindManifestName(cwd string) string {
	path := getInferredManifestFilePath(cwd)
	if path == "" {
		return ""
	}
	name, err := filepath.Rel(cwd, path)
	if err != nil {
		return ""
	}
	return name
}
