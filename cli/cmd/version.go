package cmd

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/Masterminds/semver"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/spf13/cobra"
)

// VersionString the version of the cli
var VersionString string

//Version returns information about the binary
func Version() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("View the version of the okteto binary"),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("okteto version %s \n", VersionString)
			return nil
		},
	}
}

func upgradeAvailable() string {
	current, err := semver.NewVersion(config.VersionString)
	if err != nil {
		return ""
	}

	resp, err := http.Head("https://downloads.okteto.com/cloud/cli/okteto-Darwin-x86_64")
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

		if latest.GreaterThan(current) {
			return v
		}
	}

	return ""
}

func getUpgradeCommand() string {
	if runtime.GOOS == "windows" {
		return `wget https://downloads.okteto.com/cloud/cli/okteto-Windows-x86_64 -OutFile c:\windows\system32\okteto.exe`
	}

	return `curl https://get.okteto.com -sSfL | sh`
}
