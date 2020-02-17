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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"time"

	"github.com/Masterminds/semver"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"

	getter "github.com/hashicorp/go-getter"
)

var (
	// SyncthingURL is the path of the syncthing binary.
	SyncthingURL = map[string]string{
		"linux":   "https://downloads.okteto.com/cli/syncthing/1.3.4/syncthing-Linux-x86_64.tar.gz",
		"arm64":   "https://downloads.okteto.com/cli/syncthing/1.3.4/syncthing-Linux-arm64.tar.gz",
		"darwin":  "https://downloads.okteto.com/cli/syncthing/1.3.4/syncthing-Darwin-x86_64.tar.gz",
		"windows": "https://downloads.okteto.com/cli/syncthing/1.3.4/syncthing-Windows-x86_64.tar.gz",
	}

	syncthingVersion = semver.MustParse("1.3.4")
	versionRegex     = regexp.MustCompile(`syncthing v(\d+\.\d+\.\d+) .*`)
)

func syncthingExists() bool {
	_, err := os.Stat(syncthing.GetInstallPath())
	if os.IsNotExist(err) {
		return false
	}

	return true
}

func syncthingUpgradeAvailable() bool {
	if !syncthingExists() {
		return true
	}

	current := getCurrentSyncthingVersion()
	if current == nil {
		return false
	}

	log.Infof("current: %s, expected: %s", current.String(), syncthingVersion.String())
	return syncthingVersion.GreaterThan(current)
}

func getCurrentSyncthingVersion() *semver.Version {
	cmd := exec.Command(syncthing.GetInstallPath(), "--version")
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("failed to get the current syncthing version `%s`: %s", output, err)
		return nil
	}

	found := versionRegex.FindSubmatch(output)
	if len(found) < 2 {
		log.Errorf("failed to extract the version from `%s`", output)
	}

	s, err := semver.NewVersion(string(found[1]))
	if err != nil {
		log.Errorf("failed to parse the current syncthing version `%s`: %s", found, err)
		return nil
	}

	return s
}

func getSyncthingURL() (string, error) {
	log.Debugf("downloading syncthing for %s/%s", runtime.GOOS, runtime.GOARCH)

	src, ok := SyncthingURL[runtime.GOOS]
	if !ok {
		return "", fmt.Errorf("%s is not a supported platform", runtime.GOOS)
	}

	if runtime.GOOS == "linux" {
		switch runtime.GOARCH {
		case "arm":
			src = SyncthingURL["arm"]
		case "arm64":
			src = SyncthingURL["arm64"]
		}
	}

	return src, nil
}

func downloadSyncthing() error {
	opts := []getter.ClientOption{getter.WithProgress(defaultProgressBar)}
	src, err := getSyncthingURL()
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile("", "")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %s", f.Name())
	}

	_ = f.Close()

	defer os.Remove(f.Name())

	client := &getter.Client{
		Src:     src,
		Dst:     f.Name(),
		Mode:    getter.ClientModeFile,
		Options: opts,
	}

	log.Infof("downloading syncthing %s from %s", syncthingVersion, client.Src)
	if err := os.Remove(client.Dst); err != nil {
		log.Infof("failed to delete %s, will try to overwrite: %s", client.Dst, err)
	}

	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 3; i++ {
		err := client.Get()
		if err == nil {
			break
		}

		log.Infof("failed to download syncthing from %s: %s", client.Src, err)
		if e := os.Remove(client.Dst); e != nil {
			log.Infof("failed to delete partially downloaded %s: %s", client.Dst, e.Error())
		}

		if i == 3 {
			return err
		}

		log.Infof("retrying syncthing download from %s: %s", client.Src, err)
		<-t.C
	}

	i := syncthing.GetInstallPath()
	// skipcq GSC-G302 syncthing is a binary so it needs exec permissions
	if err := os.Chmod(client.Dst, 0700); err != nil {
		return fmt.Errorf("failed to set permissions to %s: %s", client.Dst, err)
	}

	if err := os.Rename(client.Dst, i); err != nil {
		return fmt.Errorf("failed to move %s to %s: %s", client.Dst, i, err)
	}

	log.Infof("downloaded syncthing %s to %s", syncthingVersion, client.Dst)
	return nil
}
