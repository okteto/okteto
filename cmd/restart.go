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
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/pods"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Restart restarts the pods of a given dev mode deployment
func Restart(fs afero.Fs) *cobra.Command {
	var namespace string
	var k8sContext string
	var devPath string

	cmd := &cobra.Command{
		Use:    "restart [service]",
		Short:  "Restart the deployments listed in the services field of a development container",
		Args:   utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#restart"),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if devPath != "" {
				// check that the manifest file exists
				if !filesystem.FileExistsWithFilesystem(devPath, fs) {
					return oktetoErrors.ErrManifestPathNotFound
				}

				// the Okteto manifest flag should specify a file, not a directory
				if filesystem.IsDir(devPath, fs) {
					return oktetoErrors.ErrManifestPathIsDir
				}
			}

			ctx := context.Background()

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
			manifest, err := model.GetManifestV2(manifestOpts.Filename, afero.NewOsFs())
			if err != nil {
				return err
			}

			devName := ""
			if len(args) == 1 {
				devName = args[0]
			}
			dev, err := utils.GetDevFromManifest(manifest, devName)
			if err != nil {
				if !errors.Is(err, utils.ErrNoDevSelected) {
					return err
				}
				selector := utils.NewOktetoSelector("Select which development container to restart:", "Development container")
				dev, err = utils.SelectDevFromManifest(manifest, selector, manifest.Dev.GetDevs())
				if err != nil {
					return err
				}
			}

			if len(dev.Services) == 0 {
				return oktetoErrors.ErrNoServicesinOktetoManifest
			}

			serviceName := ""
			if len(args) > 0 {
				serviceName = args[0]
			}
			err = executeRestart(ctx, dev, serviceName, namespace)
			if err != nil {
				return fmt.Errorf("failed to restart your deployments: %w", err)
			}
			analytics.TrackRestart(err == nil)

			oktetoLog.Success("Deployments restarted")

			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", "", "path to the Okteto manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the restart command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the restart command is executed")

	return cmd
}

func executeRestart(ctx context.Context, dev *model.Dev, serviceName, namespace string) error {
	oktetoLog.Infof("restarting services")
	client, _, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	oktetoLog.Spinner("Restarting deployments...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		exit <- pods.Restart(ctx, dev, namespace, client, serviceName)
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
