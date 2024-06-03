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

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/apps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type analyticsTrackerInterface interface {
	TrackDown(bool)
	TrackDownVolumes(bool)
}

// Down deactivates the development container
func Down(at analyticsTrackerInterface, k8sLogsCtrl *io.K8sLogger) *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var rm bool
	var all bool

	cmd := &cobra.Command{
		Use:   "down [service]",
		Short: "Deactivate your development container",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#down"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
			if devPath != "" {
				workdir := filesystem.GetWorkdirFromManifestPath(devPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				devPath = filesystem.GetManifestPathFromWorkdir(devPath, workdir)
			}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts, afero.NewOsFs())
			if err != nil {
				return err
			}

			c, _, err := okteto.GetK8sClientWithLogger(k8sLogsCtrl)
			if err != nil {
				return err
			}

			dc := down.New(afero.NewOsFs(), okteto.NewK8sClientProviderWithLogger(k8sLogsCtrl), at)

			if all {
				err := dc.AllDown(ctx, manifest, rm)
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
					options := apps.ListDevModeOn(ctx, manifest.Dev, okteto.GetContext().Namespace, c)

					if len(options) == 0 {
						oktetoLog.Success("All development containers are deactivated")
						return nil
					}
					if len(options) == 1 {
						dev = manifest.Dev[options[0]]
						err = nil
					} else {
						selector := utils.NewOktetoSelector("Select which development container to deactivate:", "Development container")
						dev, err = utils.SelectDevFromManifest(manifest, selector, options)
					}
					if err != nil {
						return err
					}
				}

				app, _, err := utils.GetApp(ctx, dev, c, false)
				if err != nil {
					return err
				}

				if apps.IsDevModeOn(app) {
					if err := dc.Down(ctx, dev, rm); err != nil {
						at.TrackDown(false)
						return fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
					}
				} else {
					oktetoLog.Success(fmt.Sprintf("Development container '%s' deactivated", dev.Name))
				}
			}

			at.TrackDown(true)
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
