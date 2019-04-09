package cmd

import "path/filepath"

func getFullPath(p string) string {
	a, _ := filepath.Abs(p)
	return a
}
