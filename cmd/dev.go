package cmd

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/up"
	"github.com/spf13/cobra"
)

func Dev(ctx context.Context) *cobra.Command {
	devCommand := &cobra.Command{
		Use:   "dev [up|down|status|doctor|exec|restart]",
		Short: "Manage your development environment",
	}

	devSubCommands := []*cobra.Command{
		up.Up(),
		Down(),
		Status(),
		Doctor(),
		Exec(),
		Restart(),
		deploy.Endpoints(ctx),
	}

	for _, subCommand := range devSubCommands {
		if subCommand.Long == "" {
			subCommand.Long = subCommand.Short
		}
		subCommand.Long = fmt.Sprintf("%s. %s", subCommand.Long, fmt.Sprintf("You can use `okteto %s` as an alias", subCommand.Use))
		devCommand.AddCommand(subCommand)
	}

	return devCommand
}
