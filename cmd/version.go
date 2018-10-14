package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These values will be stamped at build time
var (
	// VersionString is the canonical version string
	VersionString string
)

//Version returns information about the binary
func Version() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "View the version of the cnd binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("cnd version %s \n", VersionString)
			return nil
		},
	}
}
