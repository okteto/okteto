// Copyright 2021 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
)

// Down deactivates the development container
func Down() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var rm bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivates your development container",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#down"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			dev, err := utils.LoadDev(devPath, namespace, k8sContext)
			if err != nil {
				return err
			}

			if err := runDown(ctx, dev); err != nil {
				analytics.TrackDown(false)
				return err
			}

			log.Success("Development container deactivated")

			if rm {
				if err := removeVolume(ctx, dev); err != nil {
					analytics.TrackDownVolumes(false)
					return err
				}
				log.Success("Persistent volume removed")

				if os.Getenv("OKTETO_SKIP_CLEANUP") == "" {
					if err := syncthing.RemoveFolder(dev); err != nil {
						log.Infof("failed to delete existing syncthing folder")
					}
				}

				analytics.TrackDownVolumes(true)
			}

			log.Println()

			analytics.TrackDown(true)
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volume")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the down command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the down command is executed")
	return cmd
}

func runDown(ctx context.Context, dev *model.Dev) error {
	spinner := utils.NewSpinner("Deactivating your development container...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		client, _, err := k8Client.GetLocalWithContext(dev.Context)
		if err != nil {
			exit <- err
		}

		if dev.Divert != nil {
			if err := diverts.Delete(ctx, dev, client); err != nil {
				exit <- err
			}
		}

		k8sObject, err := apps.GetResource(ctx, dev, dev.Namespace, client)
		if err != nil && !errors.IsNotFound(err) {
			exit <- err
		}

		trList, err := apps.GetTranslations(ctx, dev, k8sObject, false, client)
		if err != nil {
			exit <- err
		}

		exit <- down.Run(dev, k8sObject, trList, true, client)
	}()

	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		os.Exit(130)
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func removeVolume(ctx context.Context, dev *model.Dev) error {
	spinner := utils.NewSpinner("Removing persistent volume...")
	spinner.Start()
	defer spinner.Stop()

	client, _, err := k8Client.GetLocalWithContext(dev.Context)
	if err != nil {
		return err
	}

	return volumes.Destroy(ctx, dev.GetVolumeName(), dev.Namespace, client, dev.Timeout.Default)
}
