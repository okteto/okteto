package main

import (
	"log"
	"os/exec"
	"syscall"

	"github.com/okteto/cnd/cmd"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
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
		cmd.Rm(),
		cmd.Version(),
	)

	if err := checkForGracefulExit(commands.Execute()); err != nil {
		log.Printf("ERROR: %s", err)
	}
}

func checkForGracefulExit(err error) error {
	if err == nil {
		return nil
	}

	if ce, ok := err.(*exec.ExitError); ok {
		if status, ok := ce.Sys().(syscall.WaitStatus); ok {
			// 130 is ctrl+c
			if status.ExitStatus() == 130 {
				return nil
			}
		}
	}

	return err
}
