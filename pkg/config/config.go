// Copyright 2020 The Okteto Authors
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

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	oktetoFolderName = ".okteto"
)

// VersionString the version of the cli
var VersionString string

// Config holds all the configuration values.
type Config struct {
	// HomePath is the path of the base folder for all the Okteto files
	HomePath string

	// ManifestFileName is the name of the manifest file
	ManifestFileName string
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
	home := GetUserHomeDir()
	home = filepath.Join(home, oktetoFolderName)

	if err := os.MkdirAll(home, 0700); err != nil {
		panic("failed to create the okteto directory")
	}

	return home
}

// GetDeploymentHome returns the path of the folder
func GetDeploymentHome(namespace, name string) string {
	home := GetUserHomeDir()
	home = filepath.Join(home, oktetoFolderName, namespace, name)

	if err := os.MkdirAll(home, 0700); err != nil {
		panic("failed to create the okteto deployment directory")
	}

	return home
}

// GetStateFile returns the path to the state file
func GetStateFile(namespace, name string) string {
	return filepath.Join(GetDeploymentHome(namespace, name), "okteto.state")
}

// GetUserHomeDir returns the OS home dir
func GetUserHomeDir() string {
	if v, ok := os.LookupEnv("OKTETO_HOME"); ok {
		return v
	}

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
	home := GetUserHomeDir()
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
