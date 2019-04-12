package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var (
	defaultConfig = &Config{
		FolderName:       ".okteto",
		ManifestFileName: "okteto.yml",
	}

	overrideConfig = &Config{}
)

// Config holds all the configuration values.
type Config struct {
	// HomePath is the path of the base folder for all the Okteto files
	HomePath string

	// FolderName is the name of the  folder that stores the state on the client machine
	FolderName string

	// ManifestFileName is the name of the manifest file
	ManifestFileName string
}

//SetConfig sets the configuration object to use
func SetConfig(newConfig *Config) {
	overrideConfig = newConfig
}

// FolderName returns the name of the state folder
func FolderName() string {
	if overrideConfig.FolderName == "" {
		return defaultConfig.FolderName
	}

	return overrideConfig.FolderName
}

// ManifestFileName returns the name of the manifest file
func ManifestFileName() string {
	if overrideConfig.ManifestFileName == "" {
		return defaultConfig.ManifestFileName
	}

	return overrideConfig.ManifestFileName
}

//GetBinaryName returns the name of the binary
func GetBinaryName() string {
	return filepath.Base(GetBinaryFullPath())
}

//GetBinaryFullPath returns the name of the binary
func GetBinaryFullPath() string {
	return os.Args[0]
}

// GetHome returns the path of the folder
func GetHome() string {
	var folder = defaultConfig.FolderName
	if overrideConfig.FolderName != "" {
		folder = overrideConfig.FolderName
	}

	home := getHomeDir()

	if overrideConfig.HomePath != "" {
		home = overrideConfig.HomePath
	}

	home = filepath.Join(home, folder)

	if err := os.MkdirAll(home, 0700); err != nil {
		log.Fatalf("failed to create the home directory: %s\n", err)
	}

	return home
}

// GetHomeDir returns the OS home dir
func getHomeDir() string {
	home := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
	}

	return home
}
