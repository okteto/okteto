package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/okteto/cnd/k8/exec"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/model"
	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeExec(devPath, args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", "cnd.yml", "manifest file")
	return cmd
}

func executeExec(devPath string, args []string) error {
	namespace, client, config, err := client.Get()
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	pods, err := client.CoreV1().Pods(namespace).List(v1.ListOptions{
		LabelSelector: fmt.Sprintf("cnd=%s", dev.Name),
	})

	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("cloud native environment is not initialized. Please run 'cnd up' first")
	}

	if len(pods.Items) > 1 {
		return fmt.Errorf("More than one cloud native environment have the same name. Please restart your environment")
	}

	return exec.Exec(client, config, &pods.Items[0], os.Stdin, os.Stdout, os.Stderr, args)
}
