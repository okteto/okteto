package cmd

import (
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
)

//Down deactivates the development environment
func Down() *cobra.Command {
	var devPath string
	var namespace string
	var rm bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivates your development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting down command")

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

			log.Success("Development environment deactivated")

			if rm {
				if err := cleanVolume(dev); err != nil {
					analytics.TrackDown(config.VersionString, false)
					return err
				}
				log.Success("Persistent volume cleaned")
			}

			log.Println()

			analytics.TrackDown(config.VersionString, true)
			log.Info("completed down command")
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

func runDown(dev *model.Dev) error {
	spinner := newSpinner("Deactivating your development environment...")
	spinner.start()
	defer spinner.stop()

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
	tr, err := deployments.GetTranslations(dev, d, client)
	if err != nil {
		return err
	}

	if len(tr) == 0 {
		log.Info("no translations available in the deployment")
	}

	for _, t := range tr {
		if t.Deployment == nil {
			continue
		}
		if err := deployments.DevModeOff(t.Deployment, client); err != nil {
			return err
		}
	}

	if err := secrets.Destroy(dev, client); err != nil {
		return err
	}

	if err := statefulsets.Destroy(dev, client); err != nil {
		return err
	}

	if err := removeVolumes(dev); err != nil {
		return err
	}

	stopSyncthing(dev)

	if err := ssh.RemoveEntry(dev.Name); err != nil {
		log.Infof("failed to remove ssh entry: %s", err)
	}

	if d == nil {
		return nil
	}

	if _, ok := d.Annotations[model.OktetoAutoCreateAnnotation]; ok {
		if err := deployments.Destroy(dev, client); err != nil {
			return err
		}
		if len(dev.Services) == 0 {
			if err := services.Destroy(dev, client); err != nil {
				return err
			}
		}
	}

	return nil
}

func removeVolumes(dev *model.Dev) error {
	log.Info("deleting old persistent volumes")

	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	for i := 0; i <= len(dev.Volumes); i++ {
		if err := volumes.Destroy(dev.GetVolumeName(i), dev, client); err != nil {
			return err
		}
	}

	return nil
}

func cleanVolume(dev *model.Dev) error {
	spinner := newSpinner("Cleaning volume...")
	spinner.start()
	defer spinner.stop()

	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	exists, err := volumes.Exists(dev.Namespace, client)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	if err := pods.RunCleanerPod(dev, client); err != nil {
		return err
	}
	return nil
}

func stopSyncthing(dev *model.Dev) {
	sy, err := syncthing.New(dev)
	if err != nil {
		log.Infof("failed to create syncthing instance")
		return
	}

	if err := sy.Stop(true); err != nil {
		log.Infof("failed to stop existing syncthing")
	}

	if err := sy.RemoveFolder(); err != nil {
		log.Infof("failed to delete existing syncthing folder")
	}
}
