package cmd

import (
	"fmt"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/spf13/cobra"
)

//Create creates resources
func Create() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Creates resources"),
	}
	cmd.AddCommand(namespace.Create())
	return cmd
}
