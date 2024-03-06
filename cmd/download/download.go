package download

import (
	"fmt"
	"github.com/okteto/okteto/pkg/resolve"

	"github.com/spf13/cobra"
)

var (
	targetVersion string
	channel       string
)

var Cmd = &cobra.Command{
	Use:          "download",
	Hidden:       true,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if targetVersion == "latest" || targetVersion == "" {
			latestVersion, err := resolve.FindLatest(cmd.Context(), resolve.FindLatestOptions{
				Channel: channel,
			})
			if err != nil {
				return fmt.Errorf("failed to find latest version: %v", err)
			}
			targetVersion = latestVersion
			fmt.Printf("resolved \"latest\" to %s\n", targetVersion)
		}

		fmt.Printf("Downloading %s\n", targetVersion)
		progressCh, err := resolve.Pull(cmd.Context(), targetVersion, resolve.PullOptions{
			Channel: channel,
		})
		if err != nil {
			return err
		}
		for p := range progressCh {
			fmt.Printf("  transferred %v / %v bytes (%.2f%%)\n", p.Completed, p.Size, 100*p.Progress)
			if p.Done {
				if p.Error != nil {
					fmt.Printf("download finished with error: %v\n", p.Error)
				} else {
					fmt.Printf("download complete: %s\n", p.Destination)
				}
			}
		}
		return nil
	},
}

func init() {
	Cmd.Flags().StringVar(&targetVersion, "version", "latest", "version to pull. If not defined it will the latest known version")
	Cmd.Flags().StringVar(&channel, "channel", "stable", "channel from where to resolve versions. Defaults to the stable channel")
}
