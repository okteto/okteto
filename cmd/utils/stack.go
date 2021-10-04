// Copyright 2021 The Okteto Authors
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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

var (
	//DefaultStackManifest default okteto stack manifest file
	DefaultStackManifest    = "okteto-stack.yml"
	secondaryStackManifests = [][]string{
		{"okteto-stack.yaml"},
		{"stack.yml"},
		{"stack.yaml"},
		{".okteto", "okteto-stack.yaml"},
	}
	deprecatedManifests = []string{"stack.yml", "stack.yaml"}
)

// LoadStack loads an okteto stack manifest checking "yml" and "yaml"
func LoadStack(name, stackPath string) (*model.Stack, error) {
	var isCompose bool
	if model.FileExists(stackPath) {
		if isPathAComposeFile(stackPath) {
			isCompose = true
		}
		return model.GetStack(name, stackPath, isCompose)
	}

	if stackPath == DefaultStackManifest {
		for _, secondaryStackManifest := range secondaryStackManifests {
			manifestPath := filepath.Join(secondaryStackManifest...)
			if model.FileExists(manifestPath) {
				if isDeprecatedExtension(manifestPath) {
					deprecatedFile := filepath.Base(manifestPath)
					log.Warning("The file %s will be deprecated as a default stack file name in a future version. Please consider renaming your stack file to 'okteto-stack.yml'", deprecatedFile)
				}
				if isPathAComposeFile(manifestPath) {
					isCompose = true
				}
				return model.GetStack(name, manifestPath, isCompose)
			}
		}
	}

	return nil, fmt.Errorf("'%s' does not exist", stackPath)
}

func isPathAComposeFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "docker-compose")
}

func isDeprecatedExtension(stackPath string) bool {
	base := filepath.Base(stackPath)
	for _, deprecatedManifest := range deprecatedManifests {
		if deprecatedManifest == base {
			return true
		}
	}
	return false
}
