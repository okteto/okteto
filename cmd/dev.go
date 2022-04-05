package cmd

import (
	"github.com/okteto/okteto/cmd/up"
	"github.com/spf13/cobra"
)

func Dev() *cobra.Command {
	command := &cobra.Command{
		Use:   "dev [up|down|status|doctor|exec|restart]",
		Short: "Manage your development environment",
	}

	command.AddCommand(up.Up())
	command.AddCommand(Down())
	command.AddCommand(Status())
	command.AddCommand(Doctor())
	command.AddCommand(Exec())
	command.AddCommand(Restart())

	return command
}
