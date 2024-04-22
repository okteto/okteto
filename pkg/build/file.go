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

package build

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

// CreateDockerfileWithVolumeMounts creates the Dockerfile with volume mounts and returns the BuildInfo
func CreateDockerfileWithVolumeMounts(image string, volumes []VolumeMounts, fs afero.Fs) (*Info, error) {
	build := &Info{}

	ctx, err := filepath.Abs(".")
	if err != nil {
		return build, nil
	}
	build.Context = ctx
	dockerfileTmpFolder := filepath.Join(config.GetOktetoHomeWithFilesystem(fs), ".dockerfile")
	filePerm := os.FileMode(0700)
	if err := fs.MkdirAll(dockerfileTmpFolder, filePerm); err != nil {
		return build, fmt.Errorf("failed to create %s: %w", dockerfileTmpFolder, err)
	}

	tmpFile, err := afero.TempFile(fs, dockerfileTmpFolder, "buildkit-")
	if err != nil {
		return build, err
	}

	datawriter := bufio.NewWriter(tmpFile)
	defer datawriter.Flush()

	_, err = fmt.Fprintf(datawriter, "FROM %s\n", image)
	if err != nil {
		return build, fmt.Errorf("failed to write dockerfile: %w", err)
	}
	for _, volume := range volumes {
		_, err = fmt.Fprintf(datawriter, "COPY %s %s\n", filepath.ToSlash(volume.LocalPath), volume.RemotePath)
		if err != nil {
			oktetoLog.Infof("failed to write volume %s: %s", volume.LocalPath, err)
			continue
		}
	}

	build.Dockerfile = tmpFile.Name()
	build.VolumesToInclude = volumes
	return build, nil
}
