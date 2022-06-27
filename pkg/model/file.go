// Copyright 2022 The Okteto Authors
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

package model

import (
	"os"
	"path/filepath"
)

var (
	// PipelineFiles represents the possible names an okteto pipeline can have
	PipelineFiles = []string{
		"okteto-pipeline.yml",
		"okteto-pipeline.yaml",
		"okteto-pipelines.yml",
		"okteto-pipelines.yaml",
		".okteto/okteto-pipeline.yml",
		".okteto/okteto-pipeline.yaml",
		".okteto/okteto-pipelines.yml",
		".okteto/okteto-pipelines.yaml",
	}
	// ComposeFiles represents the possible names an okteto compose can have
	ComposeFiles = []string{
		"okteto-stack.yml",
		"okteto-stack.yaml",
		"stack.yml",
		"stack.yaml",
		".okteto/okteto-stack.yml",
		".okteto/okteto-stack.yaml",
		".okteto/stack.yml",
		".okteto/stack.yaml",

		"okteto-compose.yml",
		"okteto-compose.yaml",
		".okteto/okteto-compose.yml",
		".okteto/okteto-compose.yaml",

		"docker-compose.yml",
		"docker-compose.yaml",
		".okteto/docker-compose.yml",
		".okteto/docker-compose.yaml",
	}
	// OktetoManifestFiles represents the possible names an okteto manifest can have
	OktetoManifestFiles = []string{
		"okteto.yml",
		"okteto.yaml",
		".okteto/okteto.yml",
		".okteto/okteto.yaml",
	}
	// HelmChartsSubPaths represents the possible names a helm chart can have
	HelmChartsSubPaths = []string{
		"chart",
		"charts",
		"helm/chart",
		"helm/charts",
	}
	// K8sManifestSubPaths represents the possible names a k8s manifest can have
	K8sManifestSubPaths = []string{
		"manifests",
		"manifests.yml",
		"manifests.yaml",
		"kubernetes",
		"kubernetes.yml",
		"kubernetes.yaml",
		"k8s",
		"k8s.yml",
		"k8s.yaml",
	}
)

// GetWorkdirFromManifestPath sets the path
func GetWorkdirFromManifestPath(manifestPath string) string {
	dir := filepath.Dir(manifestPath)
	if filepath.Base(dir) == ".okteto" {
		dir = filepath.Dir(dir)
	}
	return dir
}

// GetManifestPathFromWorkdir returns the path from a workdir
func GetManifestPathFromWorkdir(manifestPath, workdir string) string {
	mPath, err := filepath.Rel(workdir, manifestPath)
	if err != nil {
		return ""
	}
	return mPath
}

// FileExistsAndNotDir checks if the file exists and its not a dir
func FileExistsAndNotDir(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// GetFilePathFromWdAndFiles joins the cwd with the files and returns it if
// one of them exists and is not a directory
func GetFilePathFromWdAndFiles(cwd string, files []string) string {
	for _, name := range files {
		path := filepath.Join(cwd, name)
		if FileExistsAndNotDir(path) {
			return path
		}
	}
	return ""
}
