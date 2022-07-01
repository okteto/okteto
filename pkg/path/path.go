package path

import "path/filepath"

func GetRelativePathFromCWD(cwd string, path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return path, nil
	}

	relativePath, err := filepath.Rel(cwd, path)
	if err != nil {
		return "", err
	}
	return relativePath, nil
}
