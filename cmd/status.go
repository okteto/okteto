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
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/status"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
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
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#status"),
		RunE: func(cmd *cobra.Command, args []string) error {

			if okteto.InDevContainer() {
				return errors.ErrNotInDevContainer
			}

			dev, err := utils.LoadDev(devPath, namespace, k8sContext)
			if err != nil {
				return err
			}

			ctx := context.Background()
			waitForStates := []config.UpState{config.Synchronizing, config.Ready}
			if err := status.Wait(ctx, dev, waitForStates); err != nil {
				return err
			}

			sy, err := syncthing.Load(dev)
			if err != nil {
				log.Infof("error accessing the syncthing info file: %s", err)
				return errors.ErrNotInDevMode
			}
			if showInfo {
				log.Information("Local syncthing url: http://%s", sy.GUIAddress)
				log.Information("Remote syncthing url: http://%s", sy.RemoteGUIAddress)
				log.Information("Syncthing username: okteto")
				log.Information("Syncthing password: %s", sy.GUIPassword)
			}

			if watch {
				err = runWithWatch(ctx, dev, sy)
			} else {
				err = runWithoutWatch(ctx, dev, sy)
			}

			analytics.TrackStatus(err == nil, showInfo)
			return err
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executing")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the up command is executing")
	cmd.Flags().BoolVarP(&showInfo, "info", "i", false, "show syncthing links for troubleshooting the synchronization service")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch for changes")
	return cmd
}

func runWithWatch(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) error {
	suffix := "Synchronizing your files..."
	spinner := utils.NewSpinner(suffix)
	pbScaling := 0.30
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		ticker := time.NewTicker(1000 * time.Millisecond)
		for {
			<-ticker.C
			message := ""
			progress, err := status.Run(ctx, dev, sy)
			if err != nil {
				log.Infof("error accessing status: %s", err)
				continue
			}
			if progress == 100 {
				message = "Files synchronized"
			} else {
				message = utils.RenderProgressBar(suffix, progress, pbScaling)
			}
			spinner.Update(message)
			exit <- nil
		}
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

func runWithoutWatch(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) error {
	progress, err := status.Run(ctx, dev, sy)
	if err != nil {
		return err
	}
	if progress == 100 {
		log.Success("Synchronization status: %.2f%%", progress)
	} else {
		log.Yellow("Synchronization status: %.2f%%", progress)
	}
	return nil
}
