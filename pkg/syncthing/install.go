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

package syncthing

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	getter "github.com/hashicorp/go-getter"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

const (
	syncthingVersion       = "1.18.2"
	syncthingVersionEnvVar = "OKTETO_SYNCTHING_VERSION"
)

var (
	versionRegex       = regexp.MustCompile(`syncthing v(\d+\.\d+\.\d+)(-rc\.[0-9])?.*`)
	downloadURLFormats = map[string]string{
		"linux":       "https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-linux-amd64-v%[1]s.tar.gz",
		"arm":         "https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-linux-arm-v%[1]s.tar.gz",
		"arm64":       "https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-linux-arm64-v%[1]s.tar.gz",
		"darwinArm64": "https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-macos-arm64-v%[1]s.zip",
		"darwin":      "https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-macos-amd64-v%[1]s.zip",
		"windows":     "https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-windows-amd64-v%[1]s.zip",
	}
)

// Install installs syncthing locally
func Install(p getter.ProgressTracker) error {
	log.Infof("installing syncthing for %s/%s", runtime.GOOS, runtime.GOARCH)

	minimum := GetMinimumVersion()
	downloadURL, err := GetDownloadURL(runtime.GOOS, runtime.GOARCH, minimum.String())
	if err != nil {
		return err
	}

	opts := []getter.ClientOption{}
	if p != nil {
		opts = []getter.ClientOption{getter.WithProgress(p)}
	}

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create temp download dir")
	}

	client := &getter.Client{
		Src:     downloadURL,
		Dst:     dir,
		Mode:    getter.ClientModeDir,
		Options: opts,
	}

	defer os.RemoveAll(dir)

	if err := client.Get(); err != nil {
		return fmt.Errorf("failed to download syncthing from %s: %s", client.Src, err)
	}

	i := getInstallPath()
	b := getBinaryPathInDownload(dir, downloadURL)

	if _, err := os.Stat(b); err != nil {
		return fmt.Errorf("%s didn't include the syncthing binary: %s", downloadURL, err)
	}

	// skipcq GSC-G302 syncthing is a binary so it needs exec permissions
	if err := os.Chmod(b, 0700); err != nil {
		return fmt.Errorf("failed to set permissions to %s: %s", b, err)
	}

	if model.FileExists(i) {
		if err := os.Remove(i); err != nil {
			log.Infof("failed to delete %s, will try to overwrite: %s", i, err)
		}
	}

	if err := model.CopyFile(b, i); err != nil {
		return fmt.Errorf("failed to write %s: %s", i, err)
	}

	log.Infof("downloaded syncthing %s to %s", syncthingVersion, i)
	return nil
}

// IsInstalled returns true if syncthing is installed
func IsInstalled() bool {
	_, err := os.Stat(getInstallPath())
	return !os.IsNotExist(err)
}

// ShouldUpgrade returns true if syncthing should be upgraded
func ShouldUpgrade() bool {
	if !IsInstalled() {
		return true
	}
	current := getInstalledVersion()
	if current == nil {
		return true
	}

	minimum := GetMinimumVersion()

	return minimum.GreaterThan(current)
}

func GetMinimumVersion() *semver.Version {
	v := os.Getenv(syncthingVersionEnvVar)
	if v == "" {
		v = syncthingVersion
	}

	return semver.MustParse(v)
}

func getInstalledVersion() *semver.Version {
	cmd := exec.Command(getInstallPath(), "--version")
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("failed to get the current syncthing version `%s`: %s", output, err)
		return nil
	}

	s, err := parseVersionFromOutput(output)
	if err != nil {
		log.Errorf("failed to parse the current syncthing version `%s`: %s", output, err)
		return nil
	}

	return s
}

func parseVersionFromOutput(output []byte) (*semver.Version, error) {
	found := versionRegex.FindSubmatch(output)

	v := ""
	switch len(found) {
	case 3:
		v = fmt.Sprintf("%s%s", found[1], found[2])
	case 2:
		v = fmt.Sprintf("%s", found[1])
	default:
		return nil, fmt.Errorf("failed to extract the version from `%s`", output)
	}

	s, err := semver.NewVersion(v)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the current syncthing version `%s`: %s", v, err)
	}

	return s, nil
}

// GetDownloadURL returns the url of the syncthing package for the OS and ARCH
func GetDownloadURL(os, arch, version string) (string, error) {
	switch os {
	case "linux":
		switch arch {
		case "arm":
			return fmt.Sprintf(downloadURLFormats["arm"], version), nil
		case "arm64":
			return fmt.Sprintf(downloadURLFormats["arm64"], version), nil
		case "amd64":
			return fmt.Sprintf(downloadURLFormats["linux"], version), nil
		}
	case "darwin":
		switch arch {
		case "arm64":
			return fmt.Sprintf(downloadURLFormats["darwinArm64"], version), nil
		default:
			return fmt.Sprintf(downloadURLFormats["darwin"], version), nil

		}
	case "windows":
		return fmt.Sprintf(downloadURLFormats[os], version), nil
	}

	return "", fmt.Errorf("%s-%s is not a supported platform", os, arch)
}

func getBinaryPathInDownload(dir, url string) string {
	_, f := filepath.Split(url)
	f = strings.TrimSuffix(f, ".tar.gz")
	f = strings.TrimSuffix(f, ".zip")
	return filepath.Join(dir, f, getBinaryName())
}
