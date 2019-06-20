package cmd

import (
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	syncK8s "github.com/okteto/okteto/pkg/syncthing/k8s"
	"github.com/spf13/cobra"
)

//Down deactivates the development environment
func Down() *cobra.Command {
	var devPath string
	var removeVolumes bool
	var namespace string

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivates your Okteto Environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting down command")

			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}

			if namespace != "" {
				dev.Namespace = namespace
			}

			image := ""
			if len(args) > 0 {
				image = args[0]
			}

			analytics.TrackDown(image, VersionString)
			err = runDown(dev, image, removeVolumes)
			if err == nil {
				log.Success("Okteto Environment deactivated")
				log.Println()
				return nil
			}

			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&removeVolumes, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

func runDown(dev *model.Dev, image string, removeVolumes bool) error {
	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	progress := newProgressBar("Deactivating your Okteto Environment...")
	progress.start()
	defer progress.stop()

	if removeVolumes {
		if err := secrets.Destroy(dev, client); err != nil {
			return err
		}

		if err := syncK8s.Destroy(dev, client); err != nil {
			return err
		}

		for i := 0; i <= len(dev.Volumes); i++ {
			if err := volumes.Destroy(dev.GetVolumeName(i), dev, client); err != nil {
				return err
			}
		}
	}

	d, err := deployments.Get(dev.Name, dev.Namespace, client)
	if err == nil {
		err = deployments.DevModeOff(d, dev, image, client)
		if err != nil {
			return err
		}
	} else {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	for _, s := range dev.Services {
		d, err := deployments.Get(s.Name, dev.Namespace, client)
		if err == nil {
			err = deployments.DevModeOff(d, &s, "", client)
			if err != nil {
				return err
			}
		} else {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
