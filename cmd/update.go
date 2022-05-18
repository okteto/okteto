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

package cmd

import (
	"fmt"
	"runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/release"
	"github.com/spf13/cobra"
)

const (
	LATEST_URL   = "https://github.com/okteto/okteto/releases/latest/download"
	INSTALL_PATH = "/usr/local/bin/okteto"
)

//Update checks if there is a new version available and updates it
func UpdateDeprecated() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update Okteto CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto update' is deprecated in favor of 'okteto version update', and will be removed in a future version")
			currentVersion, err := semver.NewVersion(config.VersionString)
			if err != nil {
				return fmt.Errorf("could not retrieve version")
			}

			if isUpdateAvailable(currentVersion) {
				displayUpdateSteps()
			} else {
				oktetoLog.Success("The latest okteto version is already installed")
			}

			return nil
		},
	}
}

//isUpdateAvailable checks if there is a new version available
func isUpdateAvailable(currentVersion *semver.Version) bool {
	v, err := release.GetLatestVersion()
	if err != nil {
		oktetoLog.Infof("failed to get latest version from github: %s", err)
		return false
	}

	if len(v) > 0 {
		latest, err := semver.NewVersion(v)
		if err != nil {
			oktetoLog.Infof("failed to parse latest version '%s': %s", v, err)
			return false
		}

		if latest.GreaterThan(currentVersion) {
			oktetoLog.Infof("new version available: %s -> %s", currentVersion.String(), latest)
			return true
		}
	}

	return false
}

func displayUpdateSteps() {
	oktetoLog.Println("You can update okteto with the following:")
	switch {
	case runtime.GOOS == "darwin" || runtime.GOOS == "linux":
		oktetoLog.Print(`
# Using installation script:
curl https://get.okteto.com -sSfL | sh`)
		if runtime.GOOS == "darwin" {
			oktetoLog.Print(`

# Using brew:
brew upgrade okteto`)
		}
	case runtime.GOOS == "windows":
		oktetoLog.Print(`# Using manual installation:
1.- Download https://downloads.okteto.com/cli/okteto.exe
2.- Add downloaded file to your $PATH

# Using scoop:
scoop update okteto`)
	}
}
