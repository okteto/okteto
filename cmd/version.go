package cmd

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/Masterminds/semver"
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

	resp, err := http.Head("https://downloads.okteto.com/cli/okteto-Darwin-x86_64")
	if err != nil {
		log.Errorf("failed to get latest version: %s", err)
		return ""
	}

	defer resp.Body.Close()
	v := resp.Header.Get("x-amz-meta-version")
	if len(v) > 0 {
		latest, err := semver.NewVersion(v)
		if err != nil {
			log.Errorf("failed to parse latest version: %s", err)
			return ""
		}

		// check if it's a minor or major change, we don't notify on revision
		if shouldNotify(latest, current) {
			return v
		}
	}

	return ""
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
