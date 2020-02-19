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

	"github.com/okteto/okteto/pkg/model"
)

const (
	defaultStackManifest   = "okteto-stack.yml"
	secondaryStackManifest = "okteto-stack.yaml"
)

//LoadStack loads an okteto stack manifet checking "yml" and "yaml"
func LoadStack(stackPath string) (*model.Stack, error) {
	if !FileExists(stackPath) {
		if stackPath == defaultStackManifest {
			if FileExists(secondaryStackManifest) {
				return LoadStack(secondaryStackManifest)
			}
		}
		return nil, fmt.Errorf("'%s' does not exist", stackPath)
	}

	return model.GetStack(stackPath)
}
