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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

const (
	LATEST_URL   = "https://github.com/okteto/okteto/releases/latest/download"
	INSTALL_PATH = "/usr/local/bin/okteto"
)

//Update check if there is a new version available and updates it
func Update() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Updates okteto version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isUpdateAvailable() {
				err := updateVersion()
				if err != nil {
					return err
				}
				log.Success("The latest okteto version has been installed")
			} else {
				log.Success("The latest okteto version is already installed")
			}

			return nil
		},
	}
}

//isUpdateAvailable checks if there is a new version available
func isUpdateAvailable() bool {
	current, err := semver.NewVersion(config.VersionString)
	if err != nil {
		return false
	}

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

		if current.GreaterThan(latest) {
			log.Infof("Installing okteto version %s", latest)
			return true
		}
	}

	return false
}

//updateVersion updates the version of the current okteto binary
func updateVersion() error {
	url, err := getDownloadURL()
	if err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create temp download dir")
	}
	downloadPath := fmt.Sprintf("%s/okteto", tempDir)
	defer os.Remove(tempDir)

	if err != nil {
		return err
	}

	err = downloadLatestVersion(url, downloadPath)
	if err != nil {
		return err
	}

	err = os.Rename(downloadPath, INSTALL_PATH)
	if err != nil {
		return err
	}

	err = os.Chmod(INSTALL_PATH, 0700)
	if err != nil {
		return err
	}
	return nil
}

//getDownloadURL gets the valid url for the current OS/ARCH
func getDownloadURL() (string, error) {
	switch {
	case runtime.GOOS == "darwin":
		switch {
		case runtime.GOARCH == "x86_64" || runtime.GOARCH == "amd64":
			return fmt.Sprintf("%s/okteto-Darwin-x86_64", LATEST_URL), nil
		case runtime.GOARCH == "arm64":
			return fmt.Sprintf("%s/okteto-Darwin-arm64", LATEST_URL), nil
		default:
			return "", fmt.Errorf("The architecture (%s) is not supported by this command. Please try installing it manually", runtime.GOARCH)
		}
	case runtime.GOOS == "linux":
		switch {
		case runtime.GOARCH == "x86_64":
			return fmt.Sprintf("%s/okteto-Linux-x86_64", LATEST_URL), nil
		case runtime.GOARCH == "amd64":
			return fmt.Sprintf("%s/okteto-Linux-x86_64", LATEST_URL), nil
		case runtime.GOARCH == "armv8":
			return fmt.Sprintf("%s/okteto-Linux-arm64", LATEST_URL), nil
		case runtime.GOARCH == "aarch64":
			return fmt.Sprintf("%s/okteto-Linux-arm64", LATEST_URL), nil
		default:
			return "", fmt.Errorf("The architecture (%s) is not supported by this command. Please try installing it manually", runtime.GOARCH)
		}
	default:
		return "", fmt.Errorf("The OS (%s) is not supported by this command. Please try installing it manually", runtime.GOOS)
	}
}

//downloadLatestVersion downloads latest version from the url given into the dir given
func downloadLatestVersion(url, dir string) error {
	log.Infof("Downloading new okteto version from %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(dir)
	if err != nil {
		return err
	}

	p := mpb.New(
		mpb.WithWidth(40),
		mpb.WithRefreshRate(180*time.Millisecond),
	)

	bar := p.Add(resp.ContentLength,
		mpb.NewBarFiller(mpb.BarStyle().Rbound("|").Tip(">").Filler("-").Padding("_")),
		mpb.PrependDecorators(
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 90),
			decor.Name(" ] "),
			decor.EwmaSpeed(decor.UnitKiB, "% .2f", 60),
		),
	)
	proxyReader := bar.ProxyReader(resp.Body)
	defer proxyReader.Close()

	io.Copy(out, proxyReader)
	p.Wait()
	return nil
}
