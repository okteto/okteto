package main

import (
	"log"

	"github.com/okteto/cnd/cmd"
	"github.com/spf13/cobra"
	"github.com/vapor-ware/ksync/pkg/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	if err := cli.InitConfig("ksync"); err != nil {
		log.Fatal(err)
	}

	commands := &cobra.Command{
		Use:   "cnd COMMAND [ARG...]",
		Short: "Manage cloud native environments",
	}
	commands.AddCommand(
		cmd.Up(),
		cmd.Exec(),
		cmd.Down(),
		cmd.Rm(),
	)

	if err := commands.Execute(); err != nil {
		log.Printf("ERROR: %s", err)
	}
}
