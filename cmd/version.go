package cmd

import (
	"fmt"

	"github.com/okteto/cnd/pkg/model"
	"github.com/spf13/cobra"
)

//Version returns information about the binary
func Version() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "View the version of the cnd binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("cnd version %s \n", model.VersionString)
			return nil
		},
	}
}
