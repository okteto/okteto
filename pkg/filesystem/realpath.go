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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/afero"
)

var errRealPathNotFound = errors.New("real path not found")

// Realpath resolves the real path of a file.
func Realpath(fs afero.Fs, fname string) (string, error) {
	// Get the absolute path provided by the user
	absPath, err := filepath.Abs(fname)
	if err != nil {
		return "", err
	}

	components := strings.Split(absPath, string(os.PathSeparator))
	currentRealPath := ""

	if components[0] == "" {
		currentRealPath = string(os.PathSeparator)
		components = components[1:]
	}
	if runtime.GOOS == "windows" {
		currentRealPath, err = filepath.Abs(string(os.PathSeparator))
		if err != nil {
			return "", err
		}
		components = components[1:]
	}

	for _, component := range components {
		root := currentRealPath
		var found bool
		f, err := fs.Open(root)
		if err != nil {
			return "", errRealPathNotFound
		}
		isDir, err := afero.IsDir(fs, root)
		if err != nil {
			return "", errRealPathNotFound
		}
		if isDir {
			files, err := f.Readdir(0)
			if err != nil {
				return "", errRealPathNotFound
			}
			for _, f := range files {
				if strings.EqualFold(f.Name(), component) {
					found = true
					currentRealPath = filepath.Join(currentRealPath, f.Name())
					break
				}
			}
		}

		if !found {
			return "", errRealPathNotFound
		}
	}
	if err != nil {
		return "", err
	}

	return currentRealPath, nil
}
