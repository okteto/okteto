package cmd

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/spf13/cobra"
)

//Create creates resources
func Create(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Creates resources"),
	}
	cmd.AddCommand(namespace.Create(ctx))
	return cmd
}
