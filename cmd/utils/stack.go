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
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
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
		{"okteto-compose.yml"},
		{"okteto-compose.yaml"},
		{".okteto", "okteto-compose.yml"},
		{".okteto", "okteto-compose.yaml"},
		{"docker-compose.yml"},
		{"docker-compose.yaml"},
		{".okteto", "docker-compose.yml"},
		{".okteto", "docker-compose.yaml"},
	}
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
			return nil, oktetoErrors.UserError{
				E:    fmt.Errorf("could not detect any compose"),
				Hint: "If you have a compose file, use the flag '--file' to point to your compose file",
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

// GetStackFiles returns the list of stack files on a path
func GetStackFiles(cwd string) []string {
	result := []string{}
	paths, err := ioutil.ReadDir(cwd)
	if err != nil {
		return nil
	}
	for _, info := range paths {
		if info.IsDir() {
			continue
		}
		if strings.HasPrefix(info.Name(), "docker-compose") || strings.HasPrefix(info.Name(), "okteto-compose") || strings.HasPrefix(info.Name(), "okteto-stack") || strings.HasPrefix(info.Name(), "stack") {
			result = append(result, info.Name())
		}
	}

	if err != nil {
		log.Infof("could not get stack files: %s", err.Error())
	}
	return result

}
