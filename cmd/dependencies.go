package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/Masterminds/semver"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"

	"net/url"

	getter "github.com/hashicorp/go-getter"
)

var (
	// SyncthingURL is the path of the syncthing binary.
	SyncthingURL = map[string]string{
		"linux":   "https://github.com/syncthing/syncthing/releases/download/v1.3.0/syncthing-linux-amd64-v1.3.0.tar.gz",
		"arm64":   "https://github.com/syncthing/syncthing/releases/download/v1.3.0/syncthing-linux-arm64-v1.3.0.tar.gz",
		"darwin":  "https://github.com/syncthing/syncthing/releases/download/v1.3.0/syncthing-macos-amd64-v1.3.0.tar.gz",
		"windows": "https://github.com/syncthing/syncthing/releases/download/v1.3.0/syncthing-windows-amd64-v1.3.0.zip",
	}

	syncthingVersion = semver.MustParse("1.3.0")
	versionRegex     = regexp.MustCompile(`syncthing v(\d+\.\d+\.\d+) .*`)
)

func syncthingUpgradeAvailable() bool {
	_, err := os.Stat(syncthing.GetInstallPath())
	if os.IsNotExist(err) {
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

func getSyncthingURL(os string) (*url.URL, error) {
	log.Debugf("downloading syncthing for %s/%s", runtime.GOOS, runtime.GOARCH)

	src, ok := SyncthingURL[os]
	if !ok {
		return nil, fmt.Errorf("%s is not a supported platform", runtime.GOOS)
	}

	if os == "linux" {
		switch runtime.GOARCH {
		case "arm":
			src = SyncthingURL["arm"]
		case "arm64":
			src = SyncthingURL["arm64"]
		}
	}

	return url.Parse(src)
}

func downloadSyncthing() error {
	opts := []getter.ClientOption{
		getter.WithProgress(defaultProgressBar),
	}

	url, err := getSyncthingURL(runtime.GOOS)
	if err != nil {
		return err
	}

	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmp)

	client := &getter.Client{
		Src:     url.String(),
		Dst:     tmp,
		Mode:    getter.ClientModeDir,
		Options: opts,
	}

	log.Debugf("downloading syncthing %s from %s to %s", syncthingVersion, client.Src, client.Dst)

	if err := client.Get(); err != nil {
		log.Infof("failed to download syncthing from %s: %s", client.Src, err)
		return err
	}

	subdir, err := getter.SubdirGlob(client.Dst, "*")
	if err != nil {
		return fmt.Errorf("syncthing download is malformed: %w", err)
	}

	if err := os.Rename(filepath.Join(subdir, syncthing.GetBinaryName()), syncthing.GetInstallPath()); err != nil {
		return fmt.Errorf("failed to extract the syncthing binary to %s: %w", syncthing.GetInstallPath(), err)
	}

	if err := os.Chmod(syncthing.GetInstallPath(), 0700); err != nil {
		return err
	}

	log.Infof("downloaded syncthing %s to %s", syncthingVersion, syncthing.GetInstallPath())

	return nil
}
