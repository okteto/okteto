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

package model

import (
	"os"
	"path/filepath"
)

// IgnoreFilename is the name of the okteto ignore file
const IgnoreFilename = ".oktetoignore"

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

func UpdateCWDtoManifestPath(manifestPath string) (string, error) {
	workdir := GetWorkdirFromManifestPath(manifestPath)
	if err := os.Chdir(workdir); err != nil {
		return "", err
	}
	updatedManifestPath := GetManifestPathFromWorkdir(manifestPath, workdir)
	return updatedManifestPath, nil
}
