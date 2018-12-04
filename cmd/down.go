package cmd

import (
	"fmt"

	"github.com/okteto/cnd/storage"
	"github.com/okteto/cnd/syncthing"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/deployments"
	"github.com/okteto/cnd/model"
	"github.com/spf13/cobra"
)

//Down stops a cloud native environment
func Down() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stops a cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeDown(devPath)
		},
	}

	return cmd
}

func executeDown(devPath string) error {
	fmt.Println("Deactivating dev mode...")

	namespace, client, _, err := client.Get()
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	name, err := deployments.Deploy(dev, namespace, client)
	if err != nil {
		return err
	}

	syncthing, err := syncthing.NewSyncthing(name, namespace, dev.Mount.Source)
	if err != nil {
		return err
	}

	storage.Delete(namespace, name)

	return syncthing.Stop()
}
