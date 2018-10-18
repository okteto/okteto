package cmd

import (
	"log"

	"github.com/okteto/cnd/syncthing"
	"k8s.io/client-go/kubernetes"

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

	if err := restoreService(dev, namespace, client); err != nil {
		return err
	}

	syncthing, err := syncthing.NewSyncthing(dev.Name, namespace, dev.Mount.Source)
	if err != nil {
		return err
	}

	return syncthing.Stop()
}

func restoreService(dev *model.Dev, namespace string, client *kubernetes.Clientset) error {
	s, err := dev.Service(false)
	if err != nil {
		return err
	}

	return services.Deploy(s, namespace, client)
}
