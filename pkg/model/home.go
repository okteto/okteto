package model

import (
	"os"
	"runtime"
)

// GetHomeDir returns an OS-aware home dir
func GetHomeDir() string {
	home := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
	}

	return home
}
