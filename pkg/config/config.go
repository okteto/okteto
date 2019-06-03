package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	folderName       = ".okteto"
	manifestFileName = "okteto.yml"
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

// FolderName returns the name of the state folder
func FolderName() string {
	return folderName
}

// ManifestFileName returns the name of the manifest file
func ManifestFileName() string {
	return manifestFileName
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
	home := getHomeDir()
	home = filepath.Join(home, folderName)

	if err := os.MkdirAll(home, 0700); err != nil {
		log.Fatalf("failed to create the okteto directory: %s\n", err)
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

// GetKubeConfigFile returns the path to the kubeconfig file, taking the KUBECONFIG env var into consideration
func GetKubeConfigFile() string {
	home := getHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if len(kubeconfigEnv) > 0 {
		kubeconfig = splitKubeConfigEnv(kubeconfigEnv)
	}
	return kubeconfig
}

func splitKubeConfigEnv(value string) string {
	if runtime.GOOS == "windows" {
		return strings.Split(value, ";")[0]
	}
	return strings.Split(value, ":")[0]
}
