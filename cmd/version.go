package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"

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

	resp, err := http.Get("https://downloads.okteto.com/cli/latest")
	if err != nil {
		log.Infof("failed to get latest version: %s", err)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		log.Infof("failed to get latest version: %d", resp.StatusCode)
		return ""
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Infof("failed to read the latest version response: %s", err)
		return ""
	}

	v := string(bodyBytes)
	v = strings.TrimSuffix(v, "\n")
	if len(v) > 0 {
		latest, err := semver.NewVersion(v)
		if err != nil {
			log.Infof("failed to parse latest version '%s': %s", v, err)
			return ""
		}

		log.Infof("latest version is %s", latest.String())

		if latest.GreaterThan(current) {
			return v
		}
	}

	return ""
}

func getUpgradeCommand() string {
	if runtime.GOOS == "windows" {
		return `wget https://downloads.okteto.com/cli/okteto-Windows-x86_64 -OutFile c:\windows\system32\okteto.exe`
	}

	return `curl https://get.okteto.com -sSfL | sh`
}
