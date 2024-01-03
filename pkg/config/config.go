// Copyright 2023 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
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
	// Activating up started
	Activating UpState = "activating"
	// Starting up started the dev pod creation
	Starting = "starting"
	// Attaching up attaching volume
	Attaching = "attaching"
	// Pulling  up pulling images
	Pulling = "pulling"
	// StartingSync up preparing syncthing
	StartingSync = "startingSync"
	// Synchronizing up is syncthing
	Synchronizing = "synchronizing"
	// Ready up finished
	Ready = "ready"
	// Failed up failed
	Failed = "failed"

	stateFile string = "okteto.state"

	// OktetoContextVariableName defines the kubeconfig context of okteto commands
	OktetoContextVariableName = "OKTETO_CONTEXT"

	// OktetoDefaultSelfSignedIssuer is the self signed CA issuer name used in helm chart installs
	OktetoDefaultSelfSignedIssuer = "okteto-wildcard-ca"
)

var (
	// VersionString the version of the cli
	VersionString string
)

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
	if v, ok := os.LookupEnv(constants.OktetoFolderEnvVar); ok {
		if !filesystem.FileExists(v) {
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
func UpdateStateFile(devName, devNamespace string, state UpState) error {
	if devNamespace == "" {
		return fmt.Errorf("can't update state file, namespace is empty")
	}

	if devName == "" {
		return fmt.Errorf("can't update state file, name is empty")
	}

	s := filepath.Join(GetAppHome(devNamespace, devName), stateFile)

	oktetoLog.Infof("updating file '%s'", s)
	if err := os.WriteFile(s, []byte(state), 0600); err != nil {
		return fmt.Errorf("failed to update state file: %w", err)
	}
	oktetoLog.Infof("file '%s' updated successfully", s)

	return nil
}

// DeleteStateFile deletes the state file of a given dev environment
func DeleteStateFile(devName, devNamespace string) error {
	if devNamespace == "" {
		return fmt.Errorf("can't delete state file, namespace is empty")
	}

	if devName == "" {
		return fmt.Errorf("can't delete state file, name is empty")
	}

	s := filepath.Join(GetAppHome(devNamespace, devName), stateFile)
	return os.Remove(s)
}

// GetState returns the state of a given dev environment
func GetState(devName, devNamespace string) (UpState, error) {
	var result UpState
	if devNamespace == "" {
		return Failed, fmt.Errorf("can't update state file, namespace is empty")
	}

	if devName == "" {
		return Failed, fmt.Errorf("can't update state file, name is empty")
	}

	statePath := filepath.Join(GetAppHome(devNamespace, devName), stateFile)
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
	if v, ok := os.LookupEnv(constants.OktetoHomeEnvVar); ok {
		if !filesystem.FileExists(v) {
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

	return os.Getenv(homeEnvVar)

}

func homedirWindows() (string, error) {
	if home := os.Getenv(homeEnvVar); home != "" {
		return home, nil
	}

	if home := os.Getenv(userProfileEnvVar); home != "" {
		return home, nil
	}

	drive := os.Getenv(homeDriveEnvVar)
	path := os.Getenv(homePathEnvVar)
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
	kubeconfigEnv := os.Getenv(constants.KubeConfigEnvVar)
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

// GetDeployOrigin gets the pipeline deploy origin. This is the initiator of the
// deploy action: web, cli, github-action, etc
func GetDeployOrigin() (src string) {
	src = os.Getenv(oktetoOriginEnvVar)
	if src == "" {
		src = "cli"
	}
	// deploys within another okteto deploy take precedence as a deploy origin.
	// This is running okteto pipeline deploy as a step of another okteto deploy
	if os.Getenv(constants.OktetoWithinDeployCommandContextEnvVar) == "true" {
		src = "okteto-deploy"
	}
	return
}

func RunningInInstaller() bool {
	return os.Getenv(oktetoInInstaller) == "true"
}
