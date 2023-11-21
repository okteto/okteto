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

package remote

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/discovery"
	"github.com/spf13/afero"
)

const (
	oktetoDockerignoreName = ".oktetodeployignore"
)

func CreateDockerignoreFileWithFilesystem(cwd, tmpDir, manifestPathFlag string, fs afero.Fs) error {
	dockerignoreContent := []byte(``)
	dockerignoreFilePath := filepath.Join(cwd, oktetoDockerignoreName)
	if _, err := fs.Stat(dockerignoreFilePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

	} else {
		dockerignoreContent, err = afero.ReadFile(fs, dockerignoreFilePath)
		if err != nil {
			return err
		}
	}

	// write the content into the .dockerignore used for building the remote image
	filename := fmt.Sprintf("%s/%s", tmpDir, ".dockerignore")

	// in order to always sync the okteto manifest
	// we force to be excluded of the dockerignore file
	currentOktetoManifestFileName := manifestPathFlag
	if currentOktetoManifestFileName == "" {
		currentOktetoManifestFileName = discovery.FindManifestNameWithFilesystem(cwd, fs)
	}

	// update the content of dockerignore if we find the okteto manifest
	content := ""
	if string(dockerignoreContent) != "" {
		content = string(dockerignoreContent) + "\n"
	}
	if currentOktetoManifestFileName != "" {
		content = content + fmt.Sprintf("!%s", currentOktetoManifestFileName) + "\n"
	}

	return afero.WriteFile(fs, filename, []byte(content), 0600)
}
