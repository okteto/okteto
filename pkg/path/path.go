package path

import "path/filepath"

// GetRelativePathFromCWD returns the relative path from the cwd
func GetRelativePathFromCWD(cwd, path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return path, nil
	}

	relativePath, err := filepath.Rel(cwd, path)
	if err != nil {
		return "", err
	}
	return relativePath, nil
}
