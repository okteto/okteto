package cmd

import (
	"log"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/deployments"
	"github.com/okteto/cnd/k8/services"
	"github.com/okteto/cnd/ksync"
	"github.com/okteto/cnd/model"
	"github.com/spf13/cobra"
)

//Up starts or upgrades a cloud native environment
func Up() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Starts or upgrades a cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeUp(devPath)
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", "cnd.yml", "manifest file")
	return cmd
}

func executeUp(devPath string) error {
	log.Println("Executing up...")

	namespace, client, _, err := client.Get()
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	d, err := dev.Deployment()
	if err != nil {
		return err
	}

	err = deployments.Deploy(d, namespace, client)
	if err != nil {
		return err
	}

	s, err := dev.Service(true)
	if err != nil {
		return err
	}

	err = services.Deploy(s, namespace, client)
	if err != nil {
		return err
	}

	err = ksync.Create(dev, namespace)
	if err != nil {
		return err
	}

	err = ksync.Watch()
	if err != nil {
		return err
	}

	return nil
}
