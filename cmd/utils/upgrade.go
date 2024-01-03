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

package utils

import (
	"context"
	"fmt"
	"runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/github"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

func UpgradeAvailable() string {
	current, err := semver.NewVersion(config.VersionString)
	if err != nil {
		return ""
	}

	v, err := GetLatestVersionFromGithub()
	if err != nil {
		oktetoLog.Infof("failed to get latest version from github: %s", err)
		return ""
	}

	if len(v) > 0 {
		latest, err := semver.NewVersion(v)
		if err != nil {
			oktetoLog.Infof("failed to parse latest version '%s': %s", v, err)
			return ""
		}

		// check if it's a minor or major change, we don't notify on revision
		if ShouldNotify(latest, current) {
			return v
		}
	}

	return ""
}

// GetLatestVersionFromGithub returns the latest okteto version from GitHub
func GetLatestVersionFromGithub() (string, error) {
	client := github.NewClient(nil)
	ctx := context.Background()
	releases, _, err := client.Repositories.ListReleases(ctx, "okteto", "okteto", &github.ListOptions{PerPage: 10})
	if err != nil {
		return "", fmt.Errorf("fail to get releases from github: %w", err)
	}

	for _, r := range releases {
		if !r.GetPrerelease() && !r.GetDraft() {
			return r.GetTagName(), nil
		}
	}

	return "", fmt.Errorf("failed to find latest release")
}

func ShouldNotify(latest, current *semver.Version) bool {
	if current.GreaterThan(latest) {
		return false
	}

	// TODO: Remove once we pull latest version from downloads.okteto.com
	// and not github. Tracked by: https://github.com/okteto/okteto/issues/2775
	if latest.Prerelease() != "" {
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

func GetUpgradeCommand() string {
	if runtime.GOOS == "windows" {
		return `https://github.com/okteto/okteto/releases/latest/download/okteto.exe`
	}

	return `curl https://get.okteto.com -sSfL | sh`
}
