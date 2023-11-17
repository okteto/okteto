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
	// possibleOktetoManifestFiles represents the possible names an okteto manifest can have
	possibleOktetoManifestFiles = [][]string{
		{"okteto.yml"},
		{"okteto.yaml"},
		{".okteto", "okteto.yml"},
		{".okteto", "okteto.yaml"},
	}
)

// GetOktetoManifestPath returns an okteto manifest file if exists, error otherwise
func GetOktetoManifestPath(wd string) (string, error) {
	for _, possibleOktetoManifest := range possibleOktetoManifestFiles {
		manifestPath := filepath.Join(wd, filepath.Join(possibleOktetoManifest...))
		if filesystem.FileExists(manifestPath) {
			return manifestPath, nil
		}
	}
	return "", ErrOktetoManifestNotFound
}

// GetOktetoManifestPathWithFilesystem returns an okteto manifest file if exists, error otherwise
func GetOktetoManifestPathWithFilesystem(wd string, fs afero.Fs) (string, error) {
	for _, possibleOktetoManifest := range possibleOktetoManifestFiles {
		manifestPath := filepath.Join(wd, filepath.Join(possibleOktetoManifest...))
		if filesystem.FileExistsWithFilesystem(manifestPath, fs) {
			return manifestPath, nil
		}
	}
	return "", ErrOktetoManifestNotFound
}
