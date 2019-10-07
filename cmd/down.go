package cmd

import (
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
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

			log.Success("Okteto Environment deactivated")

			log.Println()

			analytics.TrackDown(config.VersionString, true)
			log.Info("completed down command")
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

func runDown(dev *model.Dev) error {
	spinner := newSpinner("Deactivating your Okteto Environment...")
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
