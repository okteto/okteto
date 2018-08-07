package cmd

import (
	"log"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/deployments"
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
	cmd.Flags().StringVarP(&devPath, "file", "f", "dev.yml", "dev yml file")
	return cmd
}

func executeDown(devPath string) error {
	log.Println("Executing down...")

	namespace, client, err := client.Get()
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

	log.Println("Done!")
	return nil
}
