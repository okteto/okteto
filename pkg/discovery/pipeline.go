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
	// PipelineFiles represents the possible names an okteto pipeline can have
	possibleOktetoPipelineFiles = [][]string{
		{"okteto-pipeline.yml"},
		{"okteto-pipeline.yaml"},
		{"okteto-pipelines.yml"},
		{"okteto-pipelines.yaml"},

		{".okteto", "okteto-pipeline.yml"},
		{".okteto", "okteto-pipeline.yaml"},
		{".okteto", "okteto-pipelines.yml"},
		{".okteto", "okteto-pipelines.yaml"},
	}
)

// GetOktetoPipelinePath returns an okteto pipeline file if exists, error otherwise
func GetOktetoPipelinePath(wd string) (string, error) {
	for _, possibleOktetoPipelineManifest := range possibleOktetoPipelineFiles {
		manifestPath := filepath.Join(wd, filepath.Join(possibleOktetoPipelineManifest...))
		if filesystem.FileExists(manifestPath) {
			return manifestPath, nil
		}
	}
	return "", ErrOktetoPipelineManifestNotFound
}

// GetOktetoPipelinePathWithFilesystem returns an okteto pipeline file if exists, error otherwise
func GetOktetoPipelinePathWithFilesystem(wd string, fs afero.Fs) (string, error) {
	for _, possibleOktetoPipelineManifest := range possibleOktetoPipelineFiles {
		manifestPath := filepath.Join(wd, filepath.Join(possibleOktetoPipelineManifest...))
		if filesystem.FileExistsWithFilesystem(manifestPath, fs) {
			return manifestPath, nil
		}
	}
	return "", ErrOktetoPipelineManifestNotFound
}
