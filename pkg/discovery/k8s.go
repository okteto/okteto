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

	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/spf13/afero"
)

var (
	// possibleK8sManifestSubPaths represents the possible names a k8s manifest can have
	possibleK8sManifestSubPaths = []string{
		"manifests",
		"manifests.yml",
		"manifests.yaml",
		"kubernetes",
		"kubernetes.yml",
		"kubernetes.yaml",
		"k8s",
		"k8s.yml",
		"k8s.yaml",
		"manifest",
		"manifest.yml",
		"manifest.yaml",
	}
)

// GetK8sManifestPath returns a k8s manifest file if exists, error otherwise
func GetK8sManifestPath(cwd string) (string, error) {
	// Files will be checked in the order defined in the list
	for _, name := range possibleK8sManifestSubPaths {
		path := filepath.Join(cwd, name)
		if filesystem.FileExists(path) {
			return path, nil
		}
	}
	return "", ErrK8sManifestNotFound
}

// GetK8sManifestPathWithFilesystem returns a k8s manifest file if exists, error otherwise
func GetK8sManifestPathWithFilesystem(cwd string, fs afero.Fs) (string, error) {
	// Files will be checked in the order defined in the list
	for _, name := range possibleK8sManifestSubPaths {
		path := filepath.Join(cwd, name)
		if filesystem.FileExistsWithFilesystem(path, fs) {
			return path, nil
		}
	}
	return "", ErrK8sManifestNotFound
}
