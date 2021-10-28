// Copyright 2021 The Okteto Authors
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

package cmd

import (
	"fmt"
	"runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

const (
	LATEST_URL   = "https://github.com/okteto/okteto/releases/latest/download"
	INSTALL_PATH = "/usr/local/bin/okteto"
)

//Update check if there is a new version available and updates it
func Update() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update okteto version",
		RunE: func(cmd *cobra.Command, args []string) error {
			currentVersion, err := semver.NewVersion(config.VersionString)
			if err != nil {
				return fmt.Errorf("could not retrieve version")
			}
			if isUpdateAvailable(currentVersion) {
				displayUpdateSteps()
			} else {
				log.Success("The latest okteto version is already installed")
			}

			return nil
		},
	}
}

//isUpdateAvailable checks if there is a new version available
func isUpdateAvailable(currentVersion *semver.Version) bool {
	v, err := utils.GetLatestVersionFromGithub()
	if err != nil {
		log.Infof("failed to get latest version from github: %s", err)
		return false
	}

	if len(v) > 0 {
		latest, err := semver.NewVersion(v)
		if err != nil {
			log.Infof("failed to parse latest version '%s': %s", v, err)
			return false
		}

		if latest.GreaterThan(currentVersion) {
			log.Infof("new version available: %s -> %s", currentVersion.String(), latest)
			return true
		}
	}

	return false
}

func displayUpdateSteps() {
	fmt.Println("You can update okteto with the following:")
	switch {
	case runtime.GOOS == "darwin" || runtime.GOOS == "linux":
		fmt.Print(`
# Using installation script:
curl https://get.okteto.com -sSfL | sh`)
		if runtime.GOOS == "darwin" {
			fmt.Print(`

# Using brew:
brew upgrade okteto`)
		}
	case runtime.GOOS == "windows":
		fmt.Print(`# Using manual installation:
1.- Download https://downloads.okteto.com/cli/okteto.exe
2.- Add downloaded file to your $PATH

# Using scoop:
scoop update okteto`)
	}
}
