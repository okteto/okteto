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
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/log"
)

// GetStackFiles returns the list of stack files on a path
func GetStackFiles(cwd string) []string {
	result := []string{}
	paths, err := os.ReadDir(cwd)
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
