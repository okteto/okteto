package cmd

import (
	"fmt"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
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
	if err := analytics.Disable(); err != nil {
		return err
	}

	fmt.Println("Analytics have been disabled")
	return nil
}

func enableAnalytics() error {
	if err := analytics.Enable(); err != nil {
		return err
	}

	fmt.Println("Analytics have been enabled")
	return nil
}
