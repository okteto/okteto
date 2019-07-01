package cmd

import (
	"fmt"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/spf13/cobra"
)

//Delete creates resources
func Delete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: fmt.Sprintf("Deletes resources"),
	}
	cmd.AddCommand(namespace.Delete())
	return cmd
}
