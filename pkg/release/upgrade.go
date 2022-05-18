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

package release

import (
	"bufio"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

func UpgradeAvailable() string {
	current, err := semver.NewVersion(config.VersionString)
	if err != nil {
		return ""
	}

	v, err := GetLatestVersion()
	if err != nil {
		oktetoLog.Infof("failed to get latest version: %s", err)
		return ""
	}

	fmt.Println("new version:", v)
	if true {
		oktetoLog.Fatalf("Boom")
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

func GetLatestVersion() (string, error) {
	c := http.Client{
		Timeout: 5 * time.Second,
	}
	channel, err := GetReleaseChannel()
	if err != nil {
		return "", err
	}
	uri := fmt.Sprintf("%s/cli/%s/versions", downloadsUrl, channel)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", err
	}
	oktetoLog.Debugf("starting GET request to: %s", uri)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var v string
	for scanner.Scan() {
		v = scanner.Text()
	}

	return v, scanner.Err()
}

func ShouldNotify(latest, current *semver.Version) bool {
	if current.GreaterThan(latest) {
		return false
	}

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
