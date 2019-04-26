package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/k8s/exec"
	"github.com/okteto/app/cli/pkg/k8s/pods"
	"github.com/okteto/app/cli/pkg/model"

	k8Client "github.com/okteto/app/cli/pkg/k8s/client"

	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	var devPath string
	var pod string
	cmd := &cobra.Command{
		Use:    "exec COMMAND",
		Hidden: true,
		Short:  "Execute a command in the cloud dev environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			devPath = getFullPath(devPath)

			if _, err := os.Stat(devPath); os.IsNotExist(err) {
				return fmt.Errorf("'%s' does not exist", devPath)
			}

			dev, err := model.Get(devPath)
			if err != nil {
				return err
			}

			err = executeExec(pod, dev, args)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", config.ManifestFileName(), "path to the manifest file")
	cmd.Flags().StringVarP(&pod, "pod", "p", "", "pod where it is executed")
	return cmd
}

func executeExec(pod string, dev *model.Dev, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, cfg, namespace, err := k8Client.Get()
	if err != nil {
		return err
	}

	if pod == "" {
		pod, err = pods.GetDevPod(ctx, dev, namespace, client)
		if err != nil {
			return err
		}
	}

	return exec.Exec(ctx, client, cfg, namespace, pod, "dev", true, os.Stdin, os.Stdout, os.Stderr, args)
}
