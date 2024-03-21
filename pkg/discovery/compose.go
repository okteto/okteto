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
	// possibleComposeManifests represents the possible names an okteto compose can have
	possibleComposeManifests = [][]string{
		{"okteto-stack.yml"},
		{"okteto-stack.yaml"},
		{"stack.yml"},
		{"stack.yaml"},
		{".okteto", "okteto-stack.yml"},
		{".okteto", "okteto-stack.yaml"},
		{".okteto", "stack.yml"},
		{".okteto", "stack.yaml"},

		{"okteto-compose.yml"},
		{"okteto-compose.yaml"},
		{".okteto", "okteto-compose.yml"},
		{".okteto", "okteto-compose.yaml"},

		{"compose.yml"},
		{"compose.yaml"},
		{"docker-compose.yml"},
		{"docker-compose.yaml"},
		{".okteto", "compose.yml"},
		{".okteto", "compose.yaml"},
		{".okteto", "docker-compose.yml"},
		{".okteto", "docker-compose.yaml"},
	}
)

// GetComposePath returns a compose file if exists, error otherwise
func GetComposePath(wd string) (string, error) {
	for _, possibleStackManifest := range possibleComposeManifests {
		manifestPath := filepath.Join(wd, filepath.Join(possibleStackManifest...))
		if filesystem.FileExists(manifestPath) {
			return manifestPath, nil
		}
	}
	return "", ErrComposeFileNotFound
}

// GetComposePathWithFilesystem returns a compose file if exists, error otherwise
func GetComposePathWithFilesystem(wd string, fs afero.Fs) (string, error) {
	for _, possibleStackManifest := range possibleComposeManifests {
		manifestPath := filepath.Join(wd, filepath.Join(possibleStackManifest...))
		if filesystem.FileExistsWithFilesystem(manifestPath, fs) {
			return manifestPath, nil
		}
	}
	return "", ErrComposeFileNotFound
}
