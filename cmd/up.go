package cmd

import (
	"log"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/deployments"
	"github.com/okteto/cnd/k8/forward"
	"github.com/okteto/cnd/k8/services"
	"github.com/okteto/cnd/syncthing"

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

	namespace, client, restConfig, err := client.Get()
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

	pod, err := getCNDPod(client, namespace, dev)
	if err != nil {
		return err
	}

	sy, err := syncthing.NewSyncthing(dev.Name, namespace, dev.Mount.Source)
	if err != nil {
		return err
	}
	pf, err := forward.NewCNDPortForward(sy.RemoteAddress)
	if err != nil {
		return err
	}

	if err := sy.Run(); err != nil {
		return err
	}

	defer pf.Stop()
	return pf.Start(client, restConfig, pod)
}
