package cmd

import (
	"log"

	"github.com/okteto/cnd/syncthing"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/services"
	"github.com/okteto/cnd/model"
	"github.com/spf13/cobra"
)

//Down stops a cloud native environment
func Down() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stops a cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeDown(devPath)
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", "cnd.yml", "manifest file")
	return cmd
}

func executeDown(devPath string) error {
	log.Println("Executing down...")

	namespace, client, _, err := client.Get()
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	s, err := dev.Service(false)
	if err != nil {
		return err
	}

	err = services.Deploy(s, namespace, client)
	if err != nil {
		return err
	}

	syncthing, err := syncthing.NewSyncthing(dev.Name, namespace, dev.Mount.Source)
	if err != nil {
		return err
	}

	return syncthing.Stop()
}
