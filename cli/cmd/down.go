package cmd

import (
	"fmt"
	"os"

	"github.com/okteto/app/cli/pkg/analytics"
	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/errors"
	k8Client "github.com/okteto/app/cli/pkg/k8s/client"
	"github.com/okteto/app/cli/pkg/k8s/deployments"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
	"github.com/spf13/cobra"
)

//Down deactivates the development environment
func Down() *cobra.Command {
	var devPath string
	var namespace string

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivates your Okteto Environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting down command")
			devPath = getFullPath(devPath)

			if _, err := os.Stat(devPath); os.IsNotExist(err) {
				return fmt.Errorf("'%s' does not exist", devPath)
			}

			dev, err := model.Get(devPath)
			if err != nil {
				return err
			}

			if namespace != "" {
				dev.Space = namespace
			}

			image := ""
			if len(args) > 0 {
				image = args[0]
			}

			analytics.TrackDown(image, VersionString)
			err = runDown(dev, image)
			if err == nil {
				log.Success("Your Okteto Environment has been deactivated")
				log.Println()
				return nil
			}

			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", config.ManifestFileName(), "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

func runDown(dev *model.Dev, image string) error {
	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Space == "" {
		dev.Space = namespace
	}

	d, err := deployments.Get(dev.Space, dev.Name, client)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	progress := newProgressBar("Deactivating your Okteto Environment...")
	progress.start()
	err = deployments.DevModeOff(d, dev, image, client)
	progress.stop()
	if err != nil {
		return err
	}

	return nil
}
