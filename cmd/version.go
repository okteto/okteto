package cmd

import (
	"fmt"

	"github.com/okteto/cnd/pkg/config"
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
