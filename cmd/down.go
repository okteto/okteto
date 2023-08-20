// Copyright 2023 The Okteto Authors
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
	"errors"
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
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/keda"
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
	var all bool

	cmd := &cobra.Command{
		Use:   "down [svc]",
		Short: "Deactivate your development container",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#down"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
			if devPath != "" {
				workdir := model.GetWorkdirFromManifestPath(devPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				devPath = model.GetManifestPathFromWorkdir(devPath, workdir)
			}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts)
			if err != nil {
				return err
			}

			c, _, err := okteto.GetK8sClient()
			if err != nil {
				return err
			}

			if all {
				err := allDown(ctx, manifest, rm)
				if err != nil {
					return err
				}

				oktetoLog.Success("All development containers are deactivated")
				return nil
			} else {
				devName := ""
				if len(args) == 1 {
					devName = args[0]
				}
				dev, err := utils.GetDevFromManifest(manifest, devName)
				if err != nil {
					if !errors.Is(err, utils.ErrNoDevSelected) {
						return err
					}
					selector := utils.NewOktetoSelector("Select which development container to deactivate:", "Development container")
					options := apps.ListDevModeOn(ctx, manifest, c)

					if len(options) == 0 {
						oktetoLog.Success("All development containers are deactivated")
						return nil
					}
					dev, err = utils.SelectDevFromManifest(manifest, selector, options)
					if err != nil {
						return err
					}
				}

				app, _, err := utils.GetApp(ctx, dev, c, false)
				if err != nil {
					return err
				}

				if apps.IsDevModeOn(app) {
					if err := runDown(ctx, dev, rm); err != nil {
						analytics.TrackDown(false)
						return fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
					}
				} else {
					oktetoLog.Success(fmt.Sprintf("Development container '%s' deactivated", dev.Name))
				}
			}

			analytics.TrackDown(true)
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volume")
	cmd.Flags().BoolVarP(&all, "all", "A", false, "deactivate all running dev containers")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the down command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the down command is executed")
	return cmd
}

func allDown(ctx context.Context, manifest *model.Manifest, rm bool) error {
	oktetoLog.Spinner("Deactivating your development containers...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if len(manifest.Dev) == 0 {
		return fmt.Errorf("okteto manifest has no 'dev' section. Configure it with 'okteto init'")
	}

	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	for _, dev := range manifest.Dev {
		app, _, err := utils.GetApp(ctx, dev, c, false)
		if err != nil {
			return err
		}

		if apps.IsDevModeOn(app) {
			oktetoLog.StopSpinner()
			if err := runDown(ctx, dev, rm); err != nil {
				analytics.TrackDown(false)
				return fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
			}
		}
	}

	analytics.TrackDown(true)
	return nil
}

func runDown(ctx context.Context, dev *model.Dev, rm bool) error {
	oktetoLog.Spinner(fmt.Sprintf("Deactivating '%s' development container...", dev.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		c, restConfig, err := okteto.GetK8sClient()
		if err != nil {
			exit <- err
			return
		}

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

		trMap, err := apps.GetTranslations(ctx, dev, app, false, c)
		if err != nil {
			exit <- err
			return
		}

		if err := down.Run(dev, app, trMap, true, c); err != nil {
			exit <- err
			return
		}

		if dev.Keda {
			keda.UnpauseKeda(app, restConfig)
		}

		oktetoLog.Success(fmt.Sprintf("Development container '%s' deactivated", dev.Name))

		if !rm {
			exit <- nil
			return
		}

		oktetoLog.Spinner(fmt.Sprintf("Removing '%s' persistent volume...", dev.Name))
		if err := removeVolume(ctx, dev); err != nil {
			analytics.TrackDownVolumes(false)
			exit <- err
			return
		}
		oktetoLog.Success(fmt.Sprintf("Persistent volume '%s' removed", dev.Name))

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
		oktetoLog.StopSpinner()
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
