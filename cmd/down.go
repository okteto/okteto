package cmd

import (
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
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
	var rm bool
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

			if err := runDown(dev); err != nil {
				analytics.TrackDown(config.VersionString, false)
				return err
			}

			log.Success("Okteto Environment deactivated")

			if rm {
				if err := removeVolumes(dev); err != nil {
					analytics.TrackDown(config.VersionString, false)
					return err
				}

				log.Success("Persistent volume deleted")
			}

			log.Println()

			analytics.TrackDown(config.VersionString, true)

			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

func runDown(dev *model.Dev) error {
	progress := newProgressBar("Deactivating your Okteto Environment...")
	progress.start()
	defer progress.stop()

	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	d, err := deployments.Get(dev, dev.Namespace, client)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	tr, err := deployments.GetTranslations(dev, d, "", client)
	if err != nil {
		return err
	}

	for _, t := range tr {
		if t.Deployment == nil {
			continue
		}
		if err := deployments.DevModeOff(t.Deployment, client); err != nil {
			return err
		}
	}

	return nil
}

func removeVolumes(dev *model.Dev) error {
	progress := newProgressBar("Deleting your persistent volume...")
	progress.start()
	defer progress.stop()

	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

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

	return nil
}
