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

package cmd

import (
	"context"
	"fmt"
	"runtime"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/v28/github"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

//Version returns information about the binary
func Version() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("View the version of the okteto binary"),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("okteto version %s \n", config.VersionString)
			return nil
		},
	}
}

func upgradeAvailable() string {
	current, err := semver.NewVersion(config.VersionString)
	if err != nil {
		return ""
	}

	v, err := getVersion()
	if err != nil {
		log.Info(err)
	}

	log.Debugf("latest version: %s", v)

	if len(v) > 0 {
		latest, err := semver.NewVersion(v)
		if err != nil {
			log.Infof("failed to parse latest version '%s': %s", v, err)
			return ""
		}

		// check if it's a minor or major change, we don't notify on revision
		if shouldNotify(latest, current) {
			return v
		}
	}

	return ""
}

func getVersion() (string, error) {
	client := github.NewClient(nil)
	ctx := context.Background()
	releases, _, err := client.Repositories.ListReleases(ctx, "okteto", "okteto", &github.ListOptions{PerPage: 5})
	if err != nil {
		return "", fmt.Errorf("fail to get releases from github: %s", err)
	}

	for _, r := range releases {
		if !r.GetPrerelease() && !r.GetDraft() {
			return r.GetTagName(), nil
		}
	}

	return "", fmt.Errorf("failed to find latest release")
}

func shouldNotify(latest, current *semver.Version) bool {
	if current.GreaterThan(latest) {
		return false
	}

	// check if it's a minor or major change, we don't notify on patch
	if latest.Major() > current.Major() {
		return true
	}

	if latest.Minor() > current.Minor() {
		return true
	}

	return false
}

func getUpgradeCommand() string {
	if runtime.GOOS == "windows" {
		return `https://github.com/okteto/okteto/releases/latest/download/okteto.exe`
	}

	return `curl https://get.okteto.com -sSfL | sh`
}
