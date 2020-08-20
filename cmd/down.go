// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
)

//Down deactivates the development container
func Down() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var rm bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivates your development container",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting down command")
			ctx := context.Background()
			dev, err := utils.LoadDev(devPath)
			if err != nil {
				return err
			}

			dev.UpdateContext(namespace, k8sContext)

			if err := runDown(dev); err != nil {
				analytics.TrackDown(false)
				return err
			}

			log.Success("Development container deactivated")
			log.Information("Run 'okteto push' to deploy your code changes to the cluster")

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
			log.Info("completed down command")
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volume")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the down command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the down command is executed")
	return cmd
}

func runDown(dev *model.Dev) error {
	spinner := utils.NewSpinner("Deactivating your development container...")
	spinner.Start()
	defer spinner.Stop()

	client, _, namespace, err := k8Client.GetLocal(dev.Context)
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

	trList, err := deployments.GetTranslations(dev, d, client)
	if err != nil {
		return err
	}

	err = down.Run(dev, d, trList, true, client)
	if err != nil {
		return err
	}

	return nil
}

func removeVolume(ctx context.Context, dev *model.Dev) error {
	spinner := utils.NewSpinner("Removing persistent volume...")
	spinner.Start()
	defer spinner.Stop()

	client, _, namespace, err := k8Client.GetLocal(dev.Context)
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	return volumes.Destroy(ctx, dev, client)
}
