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
	"fmt"
	"os"
	"os/signal"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			if err := contextCMD.Init(ctx); err != nil {
				return err
			}

			dev, err := utils.LoadDev(devPath, namespace, k8sContext)
			if err != nil {
				return err
			}

			if err := okteto.SetCurrentContext(dev.Context, dev.Namespace); err != nil {
				return err
			}

			if err := runDown(ctx, dev, rm); err != nil {
				analytics.TrackDown(false)
				return err
			}

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

func runDown(ctx context.Context, dev *model.Dev, rm bool) error {
	spinner := utils.NewSpinner("Deactivating your development container...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		c, _, err := okteto.GetK8sClient()
		if err != nil {
			exit <- err
			return
		}

		if dev.Divert != nil {
			if err := diverts.Delete(ctx, dev, c); err != nil {
				exit <- err
				return
			}
		}

		spinner.Stop()
		app, _, err := utils.GetApp(ctx, dev, c)
		if err != nil {
			if !errors.IsNotFound(err) {
				exit <- err
				return
			}
			app = apps.NewDeploymentApp(deployments.Sandbox(dev))
		}
		spinner.Start()

		trMap, err := apps.GetTranslations(ctx, dev, app, false, c)
		if err != nil {
			exit <- err
			return
		}

		if err := down.Run(dev, app, trMap, true, c); err != nil {
			exit <- err
			return
		}

		if err := c.CoreV1().PersistentVolumeClaims(dev.Namespace).Delete(ctx, fmt.Sprintf(model.DeprecatedOktetoVolumeNameTemplate, dev.Name), metav1.DeleteOptions{}); err != nil {
			log.Infof("error deleting deprecated volume: %v", err)
		}

		spinner.Stop()
		log.Success("Development container deactivated")

		if !rm {
			exit <- nil
			return
		}

		spinner.Update("Removing persistent volume...")
		spinner.Start()
		if err := removeVolume(ctx, dev); err != nil {
			analytics.TrackDownVolumes(false)
			exit <- err
			return
		}
		spinner.Stop()
		log.Success("Persistent volume removed")

		if os.Getenv("OKTETO_SKIP_CLEANUP") == "" {
			if err := syncthing.RemoveFolder(dev); err != nil {
				log.Infof("failed to delete existing syncthing folder")
			}
		}

		analytics.TrackDownVolumes(true)
		exit <- nil
	}()

	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		return errors.ErrIntSig
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func removeVolume(ctx context.Context, dev *model.Dev) error {
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	return volumes.Destroy(ctx, dev.GetVolumeName(), dev.Namespace, c, dev.Timeout.Default)
}
