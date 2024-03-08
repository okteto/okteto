// Copyright 2024 The Okteto Authors
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

package filesystem

import (
	"path/filepath"
	"strings"
)

// CleanManifestPath removes the path to the manifest file, in case the command was executed from a parent or child folder
func CleanManifestPath(manifestPath string) string {
	lastFolder := filepath.Base(filepath.Dir(manifestPath))
	if lastFolder == ".okteto" {
		path := filepath.Clean(manifestPath)
		parts := strings.Split(path, string(filepath.Separator))

		return filepath.Join(parts[len(parts)-2:]...)
	} else {
		return filepath.Base(manifestPath)
	}
}
