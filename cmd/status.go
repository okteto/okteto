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
	"os"
	"os/signal"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/status"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	completedProgress = 100
)

// Status returns the status of the synchronization process
func Status() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var showInfo bool
	var watch bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status of the synchronization process",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#status"),
		RunE: func(cmd *cobra.Command, args []string) error {

			if okteto.InDevContainer() {
				return oktetoErrors.ErrNotInDevContainer
			}

			ctx := context.Background()

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts, afero.NewOsFs())
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
				selector := utils.NewOktetoSelector("Select which development container's sync status is needed:", "Development container")
				dev, err = utils.SelectDevFromManifest(manifest, selector, manifest.Dev.GetDevs())
				if err != nil {
					return err
				}
			}

			waitForStates := []config.UpState{config.Synchronizing, config.Ready}
			if err := status.Wait(dev, waitForStates); err != nil {
				return err
			}

			sy, err := syncthing.Load(dev)
			if err != nil {
				oktetoLog.Infof("error accessing the syncthing info file: %s", err)
				return oktetoErrors.ErrNotInDevMode
			}
			if showInfo {
				oktetoLog.Information("Local syncthing url: http://%s", sy.GUIAddress)
				oktetoLog.Information("Remote syncthing url: http://%s", sy.RemoteGUIAddress)
				oktetoLog.Information("Syncthing username: okteto")
				oktetoLog.Information("Syncthing password: %s", sy.GUIPassword)
			}

			if watch {
				err = runWithWatch(ctx, sy)
			} else {
				err = runWithoutWatch(ctx, sy)
			}

			analytics.TrackStatus(err == nil, showInfo)
			return err
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executing")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the up command is executing")
	cmd.Flags().BoolVarP(&showInfo, "info", "i", false, "show syncthing links for troubleshooting the synchronization service")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch for changes")
	return cmd
}

func runWithWatch(ctx context.Context, sy *syncthing.Syncthing) error {
	textSpinner := "Synchronizing your files..."
	oktetoLog.Spinner(textSpinner)
	pbScaling := 0.30
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		ticker := time.NewTicker(1000 * time.Millisecond)
		for {
			<-ticker.C
			message := ""
			progress, err := status.Run(ctx, sy)
			if err != nil {
				oktetoLog.Infof("error accessing status: %s", err)
				continue
			}
			if progress == completedProgress {
				message = "Files synchronized"
			} else {
				message = utils.RenderProgressBar(textSpinner, progress, pbScaling)
			}
			oktetoLog.Spinner(message)
		}
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

func runWithoutWatch(ctx context.Context, sy *syncthing.Syncthing) error {
	progress, err := status.Run(ctx, sy)
	if err != nil {
		return err
	}
	if progress == completedProgress {
		oktetoLog.Success("Synchronization status: %.2f%%", progress)
	} else {
		oktetoLog.Yellow("Synchronization status: %.2f%%", progress)
	}
	return nil
}
