// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

var (
	//DefaultStackManifest default okteto stack manifest file
	DefaultStackManifest    = "okteto-stack.yml"
	secondaryStackManifests = []string{"okteto-stack.yaml", "stack.yml", "stack.yaml", "docker-compose.yml", "docker-compose.yaml"}
	deprecatedManifests     = []string{"stack.yml", "stack.yaml"}
)

//LoadStack loads an okteto stack manifest checking "yml" and "yaml"
func LoadStack(name, stackPath string) (*model.Stack, error) {
	if model.FileExists(stackPath) {
		return model.GetStack(name, stackPath)
	}

	if stackPath == DefaultStackManifest {
		for _, secondaryStackManifest := range secondaryStackManifests {
			if model.FileExists(secondaryStackManifest) {
				if isDeprecatedExtension(stackPath) {
					log.Yellow("The stack name that you are using will be deprecated in a future version. Pleas consider using one of the accepted ones.")
					log.Yellow("More information is available here: https://okteto.com/docs/reference/cli#destroy-1")
				}
				return model.GetStack(name, secondaryStackManifest)
			}
		}
	}

	composeFile := model.GetFileByRegex("docker-compose.*")
	if composeFile != "" {
		log.Yellow("Using %s as compose file. If you want to specify other compose file, you can do it by using --file flag.", composeFile)
		return model.GetStack(name, composeFile)
	}
	// TODO: Get file starting with docker-compose.*
	return nil, fmt.Errorf("'%s' does not exist", stackPath)

}

func isDeprecatedExtension(stackPath string) bool {
	for _, deprecatedManifest := range deprecatedManifests {
		if deprecatedManifest == stackPath {
			return true
		}
	}
	return false
}
