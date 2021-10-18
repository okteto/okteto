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
	possibleStackManifests = [][]string{
		{"okteto-stack.yml"},
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

//LoadStackContext loads the namespace and context of an okteto stack manifest
func LoadStackContext(stackPaths []string) (*model.ContextResource, error) {
	ctxResource := &model.ContextResource{}
	found := false
	var err error
	if len(stackPaths) == 0 {
		for _, possibleStackManifest := range possibleStackManifests {
			manifestPath := filepath.Join(possibleStackManifest...)
			if model.FileExists(manifestPath) {
				ctxResource, err = model.GetContextResource(manifestPath)
				if err != nil {
					return nil, err
				}
				found = true
				break
			}
		}
		if !found {
			return nil, errors.UserError{
				E:    fmt.Errorf("could not detect any stack file to deploy"),
				Hint: "Try setting the flag '--file' pointing to your stack file",
			}
		}
	}
	for _, stackPath := range stackPaths {
		if !model.FileExists(stackPath) {
			return nil, fmt.Errorf("'%s' does not exist", stackPath)
		}
		thisCtxResource, err := model.GetContextResource(stackPath)
		if err != nil {
			return nil, err
		}
		if thisCtxResource.Context != "" {
			ctxResource.Context = thisCtxResource.Context
		}
		if thisCtxResource.Namespace != "" {
			ctxResource.Namespace = thisCtxResource.Namespace
		}
	}
	return ctxResource, nil
}

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

func getOverrideFile(stackPath string) (*model.Stack, error) {
	extension := filepath.Ext(stackPath)
	fileName := strings.TrimSuffix(stackPath, extension)
	overridePath := fmt.Sprintf("%s.override%s", fileName, extension)
	var isCompose bool
	if model.FileExists(stackPath) {
		if isPathAComposeFile(stackPath) {
			isCompose = true
		}
		stack, err := model.GetStack("", overridePath, isCompose)
		if err != nil {
			return nil, err
		}
		return stack, nil
	}
	return nil, fmt.Errorf("override file not found")
}
func inferStack(name string) (*model.Stack, error) {
	for _, possibleStackManifest := range possibleStackManifests {
		manifestPath := filepath.Join(possibleStackManifest...)
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
	overrideStack, err := getOverrideFile(manifestPath)
	if err == nil {
		log.Info("override file detected. Merging it")
		stack = stack.Merge(overrideStack)
	}
	return stack, nil
}
