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

	"github.com/okteto/okteto/pkg/errors"
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
		{".okteto", "okteto-stack.yml"},
		{".okteto", "okteto-stack.yaml"},
		{"docker-compose.yml"},
		{"docker-compose.yaml"},
		{".okteto", "docker-compose.yml"},
		{".okteto", "docker-compose.yaml"},
	}
	deprecatedManifests = []string{"stack.yml", "stack.yaml"}
)

// LoadStack loads an okteto stack manifest checking "yml" and "yaml"
func LoadStack(name string, stackPaths []string) (*model.Stack, error) {
	var resultStack *model.Stack

	if len(stackPaths) == 0 {
		stack, err := inferStack(name)
		if err != nil {
			return nil, err
		}
		resultStack = resultStack.Merge(stack)
	} else {
		for _, stackPath := range stackPaths {
			if model.FileExists(stackPath) {
				stack, err := getStack(name, stackPath)
				if err != nil {
					return nil, err
				}

				resultStack = resultStack.Merge(stack)
				continue
			}
			return nil, fmt.Errorf("'%s' does not exist", stackPath)
		}
	}

	if err := resultStack.Validate(); err != nil {
		return nil, err
	}
	return resultStack, nil
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

func inferStack(name string) (*model.Stack, error) {
	if model.FileExists(DefaultStackManifest) {
		stack, err := getStack(name, DefaultStackManifest)
		if err != nil {
			return nil, err
		}
		return stack, nil
	}
	for _, secondaryStackManifest := range secondaryStackManifests {
		manifestPath := filepath.Join(secondaryStackManifest...)
		if model.FileExists(manifestPath) {
			stack, err := getStack(name, manifestPath)
			if err != nil {
				return nil, err
			}
			return stack, nil
		}
	}
	return nil, errors.UserError{
		E:    fmt.Errorf("could not detect any stack file to deploy."),
		Hint: "Try setting the flag '--file' pointing to your stack file",
	}
}

func getStack(name, manifestPath string) (*model.Stack, error) {
	var isCompose bool
	if isDeprecatedExtension(manifestPath) {
		deprecatedFile := filepath.Base(manifestPath)
		log.Warning("The file %s will be deprecated as a default stack file name in a future version. Please consider renaming your stack file to 'okteto-stack.yml'", deprecatedFile)
	}
	if isPathAComposeFile(manifestPath) {
		isCompose = true
	}
	stack, err := model.GetStack(name, manifestPath, isCompose)
	if err != nil {
		return nil, err
	}
	return stack, nil
}
