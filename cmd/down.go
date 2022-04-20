// Copyright 2022 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
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
		Use:   "down [svc]",
		Short: "Deactivate your development container",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#down"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
			if devPath != "" {
				workdir := utils.GetWorkdirFromManifestPath(devPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				devPath = utils.GetManifestPathFromWorkdir(devPath, workdir)
			}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts)
			if err != nil {
				return err
			}

			devName := ""
			if len(args) == 1 {
				devName = args[0]
			}
			dev, err := utils.GetDevFromManifest(manifest, devName)
			if err != nil {
				return err
			}

			if err := runDown(ctx, dev, rm); err != nil {
				analytics.TrackDown(false)
				err = fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
				return err
			}

			analytics.TrackDown(true)
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
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

		app, _, err := utils.GetApp(ctx, dev, c, false)
		if err != nil {
			if !oktetoErrors.IsNotFound(err) {
				exit <- err
				return
			}
			app = apps.NewDeploymentApp(deployments.Sandbox(dev))
		}
		if dev.Autocreate {
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

		spinner.Stop()
		oktetoLog.Success("Development container deactivated")

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
		oktetoLog.Success("Persistent volume removed")

		if os.Getenv(model.OktetoSkipCleanupEnvVar) == "" {
			if err := syncthing.RemoveFolder(dev); err != nil {
				oktetoLog.Infof("failed to delete existing syncthing folder")
			}
		}

		analytics.TrackDownVolumes(true)
		exit <- nil
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
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
