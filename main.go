package main

import (
	"log"

	"github.com/okteto/cnd/cmd"
	"github.com/spf13/cobra"
)

func main() {
	commands := &cobra.Command{
		Use:   "cnd COMMAND [ARG...]",
		Short: "Manage cloud native environments",
	}
	commands.AddCommand(
		cmd.Up(),
		cmd.Exec(),
		cmd.Down(),
	)

	if err := commands.Execute(); err != nil {
		log.Printf("ERROR: %s", err)
	}
}
