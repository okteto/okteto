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
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/pods"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"

	"github.com/okteto/okteto/pkg/model"

	"github.com/spf13/cobra"
)

// Restart restarts the pods of a given dev mode deployment
func Restart() *cobra.Command {
	var namespace string
	var k8sContext string
	var devPath string

	cmd := &cobra.Command{
		Use:    "restart [svc]",
		Short:  "Restart the deployments listed in the services field of a development container",
		Args:   utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#restart"),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
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

			if len(dev.Services) == 0 {
				return oktetoErrors.ErrNoServicesinOktetoManifest
			}

			serviceName := ""
			if len(args) > 0 {
				serviceName = args[0]
			}
			err = executeRestart(ctx, dev, serviceName)
			if err != nil {
				return fmt.Errorf("failed to restart your deployments: %s", err)
			}
			analytics.TrackRestart(err == nil)

			oktetoLog.Success("Deployments restarted")

			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the restart command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the restart command is executed")

	return cmd
}

func executeRestart(ctx context.Context, dev *model.Dev, sn string) error {
	oktetoLog.Infof("restarting services")
	client, _, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	spinner := utils.NewSpinner("Restarting deployments...")
	spinner.Start()
	defer spinner.Stop()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		exit <- pods.Restart(ctx, dev, client, sn)
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
