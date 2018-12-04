package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/okteto/cnd/cmd"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.InfoLevel)
}

func main() {
	commands := &cobra.Command{
		Use:   "cnd COMMAND [ARG...]",
		Short: "Manage cloud native environments",
	}
	commands.AddCommand(
		cmd.Up(),
		cmd.Exec(),
		cmd.Down(),
		cmd.Rm(),
		cmd.Version(),
	)

	if err := commands.Execute(); err != nil {
		log.Error(err)
	}
}
