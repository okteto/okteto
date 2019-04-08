package cmd

import (
	"fmt"

	"cli/cnd/pkg/config"
	"cli/cnd/pkg/log"
	"net/http"

	"github.com/Masterminds/semver"
	"github.com/spf13/cobra"
)

//Version returns information about the binary
func Version() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("View the version of the %s binary", config.GetBinaryName()),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("%s version %s \n", config.GetBinaryName(), config.VersionString)
			return nil
		},
	}
}

func upgradeAvailable() string {
	current, err := semver.NewVersion(config.VersionString)
	if err != nil {
		return ""
	}

	resp, err := http.Head("https://downloads.okteto.com/cli/okteto-darwin-amd64")
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
