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

package utils

import (
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/discovery"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/model"
)

// LoadStackContext loads the namespace and context of an okteto stack manifest
func LoadStackContext(stackPaths []string) (*model.ContextResource, error) {
	ctxResource := &model.ContextResource{}
	if len(stackPaths) == 0 {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		composePath, err := discovery.GetComposePath(dir)
		if err != nil {
			return nil, err
		}
		ctxResource, err = model.GetContextResource(composePath)
		if err != nil {
			return nil, err
		}
	}
	for _, stackPath := range stackPaths {
		if !filesystem.FileExists(stackPath) {
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
