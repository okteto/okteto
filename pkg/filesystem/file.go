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

package filesystem

import (
	"io"
	"os"
	"path/filepath"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

func FileExistsWithFilesystem(name string, fs afero.Fs) bool {
	_, err := fs.Stat(name)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		oktetoLog.Infof("failed to check if %s exists: %s", name, err)
	}

	return true
}

// FileExists return true if the file exists
func FileExists(name string) bool {
	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		oktetoLog.Infof("failed to check if %s exists: %s", name, err)
	}

	return true
}

// CopyFile copies a binary between from and to
func CopyFile(from, to string) error {
	fromFile, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() {
		if err := fromFile.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", from, err)
		}
	}()

	// skipcq GSC-G302 syncthing is a binary so it needs exec permissions
	toFile, err := os.OpenFile(to, os.O_RDWR|os.O_CREATE, 0700)
	if err != nil {
		return err
	}

	defer func() {
		if err := toFile.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", to, err)
		}
	}()

	_, err = io.Copy(toFile, fromFile)
	if err != nil {
		return err
	}

	return nil
}

// FileExistsAndNotDir checks if the file exists and its not a dir
func FileExistsAndNotDir(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// GetFilePathFromWdAndFiles joins the cwd with the files and returns it if
// one of them exists and is not a directory
func GetFilePathFromWdAndFiles(cwd string, files []string) string {
	for _, name := range files {
		path := filepath.Join(cwd, name)
		if FileExistsAndNotDir(path) {
			return path
		}
	}
	return ""
}
