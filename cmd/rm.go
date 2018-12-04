package cmd

import (
	"log"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/deployments"
	"github.com/okteto/cnd/k8/services"
	"github.com/okteto/cnd/model"
	"github.com/okteto/cnd/syncthing"
	"github.com/spf13/cobra"
)

//Rm removes a cloud native environment
func Rm() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeRm(devPath)
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", "cnd.yml", "manifest file")
	return cmd
}

func executeRm(devPath string) error {
	log.Println("Executing rm...")

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

	err = deployments.Destroy(d, namespace, client)
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

	syncthing, err := syncthing.NewSyncthing(s.Name, namespace, dev.Mount.Source)
	if err != nil {
		return err
	}
	return syncthing.Stop()
}
