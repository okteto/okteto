package cmd

import (
	"github.com/okteto/cnd/pkg/analytics"
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
				return analytics.Disable()
			}

			return analytics.Enable()
		},
	}
	cmd.Flags().BoolVarP(&disable, "disable", "d", false, "disable analytics")
	return cmd
}
