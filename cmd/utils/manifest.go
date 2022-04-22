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

package utils

import (
	"path/filepath"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

// InferName infers the application name from the folder received as parameter
func InferName(cwd string) string {
	repo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	oktetoLog.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repo)
}

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
