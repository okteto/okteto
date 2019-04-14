package cmd

import (
	"fmt"

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
