package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

var netClient = &http.Client{
	Timeout: time.Second * 3,
}

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
		log.Infof("failed to get the latest version from https://downloads.okteto.com/cli/latest: %s", err)
	}

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
	resp, err := netClient.Get("https://downloads.okteto.com/cli/latest")
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to get latest version, status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	v := string(b)
	v = strings.TrimSuffix(v, "\n")
	return v, nil
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
		return `wget https://downloads.okteto.com/cli/okteto-Windows-x86_64 -OutFile c:\windows\system32\okteto.exe`
	}

	return `curl https://get.okteto.com -sSfL | sh`
}
