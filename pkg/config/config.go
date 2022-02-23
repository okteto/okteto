// Copyright 2022 The Okteto Authors
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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"gopkg.in/yaml.v2"
)

// UpState represents the state of the up command
type UpState string

const (
	deprecatedAnalyticsFile = ".noanalytics"
	analyticsFile           = "analytics.json"
	tokenFile               = ".token.json"
	contextDir              = "context"
	contextsStoreFile       = "config.json"

	oktetoFolderName = ".okteto"
	//Activating up started
	Activating UpState = "activating"
	//Starting up started the dev pod creation
	Starting = "starting"
	//Attaching up attaching volume
	Attaching = "attaching"
	//Pulling  up pulling images
	Pulling = "pulling"
	//Start ingSync up preparing syncthing
	StartingSync = "startingSync"
	//Synchronize ing up is syncthing
	Synchronizing = "synchronizing"
	//Ready up fi nished
	Ready = "ready"
	//Fai led up failed
	Failed = "failed"

	stateFile string = "okteto.state"

	//OktetoContextVariableName defines the kubeconfig context of okteto commands
	OktetoContextVariableName = "OKTETO_CONTEXT"
)

// VersionString the version of the cli
var VersionString string

// GetBinaryName returns the name of the binary
func GetBinaryName() string {
	return filepath.Base(GetBinaryFullPath())
}

// GetBinaryFullPath returns the name of the binary
func GetBinaryFullPath() string {
	return os.Args[0]
}

// GetOktetoHome returns the path of the okteto folder
func GetOktetoHome() string {
	if v, ok := os.LookupEnv(model.OktetoFolderEnvVar); ok {
		if !model.FileExists(v) {
			oktetoLog.Fatalf("OKTETO_FOLDER doesn't exist: %s", v)
		}

		return v
	}

	home := GetUserHomeDir()
	d := filepath.Join(home, oktetoFolderName)

	if err := os.MkdirAll(d, 0700); err != nil {
		oktetoLog.Fatalf("failed to create %s: %s", d, err)
	}

	return d
}

// GetNamespaceHome returns the path of the folder
func GetNamespaceHome(namespace string) string {
	okHome := GetOktetoHome()
	d := filepath.Join(okHome, namespace)

	if err := os.MkdirAll(d, 0700); err != nil {
		oktetoLog.Fatalf("failed to create %s: %s", d, err)
	}

	return d
}

// GetAppHome returns the path of the folder
func GetAppHome(namespace, name string) string {
	okHome := GetOktetoHome()
	d := filepath.Join(okHome, namespace, name)

	if err := os.MkdirAll(d, 0700); err != nil {
		oktetoLog.Fatalf("failed to create %s: %s", d, err)
	}

	return d
}

// UpdateStateFile updates the state file of a given dev environment
func UpdateStateFile(dev *model.Dev, state UpState) error {
	if dev.Namespace == "" {
		return fmt.Errorf("can't update state file, namespace is empty")
	}

	if dev.Name == "" {
		return fmt.Errorf("can't update state file, name is empty")
	}

	s := filepath.Join(GetAppHome(dev.Namespace, dev.Name), stateFile)
	if err := os.WriteFile(s, []byte(state), 0644); err != nil {
		return fmt.Errorf("failed to update state file: %s", err)
	}

	return nil
}

// DeleteStateFile deletes the state file of a given dev environment
func DeleteStateFile(dev *model.Dev) error {
	if dev.Namespace == "" {
		return fmt.Errorf("can't delete state file, namespace is empty")
	}

	if dev.Name == "" {
		return fmt.Errorf("can't delete state file, name is empty")
	}

	s := filepath.Join(GetAppHome(dev.Namespace, dev.Name), stateFile)
	return os.Remove(s)
}

// GetState returns the state of a given dev environment
func GetState(dev *model.Dev) (UpState, error) {
	var result UpState
	if dev.Namespace == "" {
		return Failed, fmt.Errorf("can't update state file, namespace is empty")
	}

	if dev.Name == "" {
		return Failed, fmt.Errorf("can't update state file, name is empty")
	}

	statePath := filepath.Join(GetAppHome(dev.Namespace, dev.Name), stateFile)
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		oktetoLog.Infof("error reading state file: %s", err.Error())
		return Failed, oktetoErrors.UserError{
			E:    fmt.Errorf("development mode isn't enabled on your deployment"),
			Hint: "Run 'okteto up' to enable it and try again",
		}
	}

	if err := yaml.Unmarshal(stateBytes, &result); err != nil {
		return Failed, err
	}

	return result, nil
}

// GetUserHomeDir returns the OS home dir
func GetUserHomeDir() string {
	if v, ok := os.LookupEnv(model.OktetoHomeEnvVar); ok {
		if !model.FileExists(v) {
			oktetoLog.Fatalf("OKTETO_HOME points to a non-existing directory: %s", v)
		}

		return v
	}

	if runtime.GOOS == "windows" {
		home, err := homedirWindows()
		if err != nil {
			oktetoLog.Fatalf("couldn't determine your home directory: %s", err)
		}

		return home
	}

	return os.Getenv(model.HomeEnvVar)

}

func homedirWindows() (string, error) {
	if home := os.Getenv(model.HomeEnvVar); home != "" {
		return home, nil
	}

	if home := os.Getenv(model.UserProfileEnvVar); home != "" {
		return home, nil
	}

	drive := os.Getenv(model.HomeDriveEnvVar)
	path := os.Getenv(model.HomePathEnvVar)
	home := drive + path
	if drive == "" || path == "" {
		return "", fmt.Errorf("HOME, HOMEDRIVE, HOMEPATH, or USERPROFILE are empty. Use $OKTETO_HOME to set your home directory")
	}

	return home, nil
}

// GetKubeconfigPath returns the path to the kubeconfig file, taking the KUBECONFIG env var into consideration
func GetKubeconfigPath() []string {
	home := GetUserHomeDir()
	kubeconfig := []string{filepath.Join(home, ".kube", "config")}
	kubeconfigEnv := os.Getenv(model.KubeConfigEnvVar)
	if len(kubeconfigEnv) > 0 {
		kubeconfig = splitKubeConfigEnv(kubeconfigEnv)
	}
	return kubeconfig
}

func splitKubeConfigEnv(value string) []string {
	if runtime.GOOS == "windows" {
		return strings.Split(value, ";")
	}
	return strings.Split(value, ":")
}

func GetTokenPathDeprecated() string {
	return filepath.Join(GetOktetoHome(), tokenFile)
}

func GetDeprecatedAnalyticsPath() string {
	return filepath.Join(GetOktetoHome(), deprecatedAnalyticsFile)
}

func GetAnalyticsPath() string {
	return filepath.Join(GetOktetoHome(), analyticsFile)
}

func GetOktetoContextFolder() string {
	return filepath.Join(GetOktetoHome(), contextDir)
}

func GetOktetoContextsStorePath() string {
	return filepath.Join(GetOktetoContextFolder(), contextsStoreFile)
}

// GetCertificatePath returns the path to the certificate of the okteto buildkit
func GetCertificatePath() string {
	return filepath.Join(GetOktetoHome(), ".ca.crt")
}
