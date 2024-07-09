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

package okteto

import (
	"fmt"
	"os"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/spf13/afero"
)

// UseManifestFlag validates the manifest path flag and changes the current working directory to the directory where the manifest is
func UseManifestFlag(fs afero.Fs, manifestPathFlag string) (string, error) {
	if manifestPathFlag == "" {
		return "", nil
	}

	result := manifestPathFlag

	// we change the working directory to the directory where the manifest is
	workdir := filesystem.GetWorkdirFromManifestPath(result)
	if err := os.Chdir(workdir); err != nil {
		return result, err
	}

	// we update the manifest path to be relative to the working directory
	result = filesystem.GetManifestPathFromWorkdir(result, workdir)

	// the Okteto manifest flag should specify a file, not a directory
	if !filesystem.FileExistsAndNotDir(result, fs) {
		return result, oktetoErrors.UserError{
			E:    fmt.Errorf("the Okteto manifest specified is a directory, please specify a file"),
			Hint: "Check the path to the Okteto manifest file",
		}

	}

	return result, nil
}
