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

	"github.com/Masterminds/semver"
	getter "github.com/hashicorp/go-getter"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

const syncthingVersion = "1.9.0"

var (
	downloadURLs = map[string]string{
		"linux":   fmt.Sprintf("https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-linux-amd64-v%[1]s.tar.gz", syncthingVersion),
		"arm64":   fmt.Sprintf("https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-linux-arm64-v%[1]s.tar.gz", syncthingVersion),
		"darwin":  fmt.Sprintf("https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-macos-amd64-v%[1]s.zip", syncthingVersion),
		"windows": fmt.Sprintf("https://github.com/syncthing/syncthing/releases/download/v%[1]s/syncthing-windows-amd64-v%[1]s.zip", syncthingVersion),
	}

	minimumVersion = semver.MustParse(syncthingVersion)
	versionRegex   = regexp.MustCompile(`syncthing v(\d+\.\d+\.\d+) .*`)
)

// Install installs syncthing locally
func Install(p getter.ProgressTracker) error {
	log.Debugf("installing syncthing for %s/%s", runtime.GOOS, runtime.GOARCH)

	downloadURL, err := GetDownloadURL(runtime.GOOS, runtime.GOARCH)
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

	return minimumVersion.GreaterThan(current)
}

func getInstalledVersion() *semver.Version {
	cmd := exec.Command(getInstallPath(), "--version")
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("failed to get the current syncthing version `%s`: %s", output, err)
		return nil
	}

	found := versionRegex.FindSubmatch(output)
	if len(found) < 2 {
		log.Errorf("failed to extract the version from `%s`", output)
		return nil
	}

	s, err := semver.NewVersion(string(found[1]))
	if err != nil {
		log.Errorf("failed to parse the current syncthing version `%s`: %s", found, err)
		return nil
	}

	return s
}

// GetDownloadURL returns the url of the syncthing package for the OS and ARCH
func GetDownloadURL(os, arch string) (string, error) {
	src, ok := downloadURLs[os]
	if !ok {
		return "", fmt.Errorf("%s is not a supported platform", os)
	}

	if os == "linux" {
		switch arch {
		case "arm":
			return downloadURLs["arm"], nil
		case "arm64":
			return downloadURLs["arm64"], nil
		}
	}

	return src, nil
}

func getBinaryPathInDownload(dir, url string) string {
	_, f := filepath.Split(url)
	f = strings.TrimSuffix(f, ".tar.gz")
	f = strings.TrimSuffix(f, ".zip")
	return filepath.Join(dir, f, getBinaryName())
}
