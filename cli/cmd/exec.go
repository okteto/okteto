package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/okteto/app/cli/pkg/k8s/exec"

	k8Client "github.com/okteto/app/cli/pkg/k8s/client"

	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud dev environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := executeExec(args)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}
	return cmd
}

func executeExec(args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, cfg, err := k8Client.Get()
	if err != nil {
		return err
	}

	namespace := "oktako"
	pod := "test-5f6b55cd84-n9lll"

	return exec.Exec(ctx, client, cfg, namespace, pod, "dev", true, os.Stdin, os.Stdout, os.Stderr, args)
}
