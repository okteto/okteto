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

package validator

import (
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/spf13/afero"
)

// FileArgumentIsNotDir validates the given file, if empty it returns nil
// errors: ErrManifestPathNotFound, ErrManifestPathIsDir
func FileArgumentIsNotDir(fs afero.Fs, file string) error {
	if file == "" {
		return nil
	}

	if !filesystem.FileExistsWithFilesystem(file, fs) {
		return oktetoErrors.ErrManifestPathNotFound
	}
	if filesystem.IsDir(file, fs) {
		return oktetoErrors.ErrManifestPathIsDir
	}

	return nil
}
