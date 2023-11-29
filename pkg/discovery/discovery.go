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
