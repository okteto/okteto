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
	"bufio"
	"io"
	"os"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

// FileExistsWithFilesystem return true if the file exists or if there is an error.
func FileExistsWithFilesystem(path string, fs afero.Fs) bool {
	_, err := fs.Stat(path)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		oktetoLog.Infof("failed to check if %s exists: %s", path, err)
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

// FileExistsAndNotDir checks if the file exists, and it's not a dir
func FileExistsAndNotDir(path string, fs afero.Fs) bool {
	info, err := fs.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// GetLastNLines returns the last N lines of a file up to a max amount of bytes
func GetLastNLines(fs afero.Fs, path string, n int, maxChunkByteSize int64) ([]string, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var offset int64
	if stat.Size() > maxChunkByteSize {
		offset = stat.Size() - maxChunkByteSize
	}

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines, nil
}
