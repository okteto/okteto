package cmd

import (
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

//Analytics turns analytics on/off
func Analytics() *cobra.Command {
	var disable bool
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Enable / Disable analytics",
		RunE: func(cmd *cobra.Command, args []string) error {
			if disable {
				return disableAnalytics()
			}

			return enableAnalytics()
		},
	}
	cmd.Flags().BoolVarP(&disable, "disable", "d", false, "disable analytics")
	return cmd
}

func disableAnalytics() error {
	if err := analytics.Disable(config.VersionString); err != nil {
		return err
	}

	log.Success("Analytics have been disabled")
	return nil
}

func enableAnalytics() error {
	if err := analytics.Enable(config.VersionString); err != nil {
		return err
	}

	log.Success("Analytics have been enabled")
	return nil
}
