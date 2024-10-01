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
