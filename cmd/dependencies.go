package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/Masterminds/semver"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/syncthing"

	getter "github.com/hashicorp/go-getter"
)

var (
	// SyncthingURL is the path of the syncthing binary.
	SyncthingURL = map[string]string{
		"linux":   "https://downloads.okteto.com/cli/syncthing/1.3.3/syncthing-Linux-x86_64",
		"arm64":   "https://downloads.okteto.com/cli/syncthing/1.3.3/syncthing-Linux-arm64",
		"darwin":  "https://downloads.okteto.com/cli/syncthing/1.3.3/syncthing-Darwin-x86_64",
		"windows": "https://downloads.okteto.com/cli/syncthing/1.3.3/syncthing-Windows-x86_64",
	}

	syncthingVersion = semver.MustParse("1.3.3")
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

	client := &getter.Client{
		Src:     src,
		Dst:     syncthing.GetInstallPath(),
		Mode:    getter.ClientModeFile,
		Options: opts,
	}

	log.Infof("downloading syncthing %s from %s", syncthingVersion, client.Src)
	if err := os.Remove(client.Dst); err != nil {
		log.Infof("failed to delete %s, will try to override: %s", client.Dst, err)
	}

	if err := client.Get(); err != nil {
		log.Infof("failed to download syncthing from %s: %s", client.Src, err)
		if e := os.Remove(client.Dst); e != nil {
			log.Infof("failed to delete partially downloaded %s: %s", client.Dst, e.Error())
		}

		return err
	}

	// skipcq GSC-G302 syncthing is a binary so it needs exec permissions
	if err := os.Chmod(client.Dst, 0700); err != nil {
		return err
	}

	log.Infof("downloaded syncthing %s to %s", syncthingVersion, client.Dst)

	return nil
}
