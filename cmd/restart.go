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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"

	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/model"

	"github.com/spf13/cobra"
)

// Restart restarts the pods of a given dev mode deployment
func Restart() *cobra.Command {
	var namespace string
	var k8sContext string
	var devPath string

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restarts the deployments listed in the services field of the okteto manifest",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#restart"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			dev, err := utils.LoadDev(devPath, namespace, k8sContext)
			if err != nil {
				return err
			}
			serviceName := ""
			if len(args) > 0 {
				serviceName = args[0]
			}
			if err := executeRestart(ctx, dev, serviceName); err != nil {
				return fmt.Errorf("failed to restart your deployments: %s", err)
			}

			log.Success("Deployments restarted")

			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the restart command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the restart command is executed")

	return cmd
}

func executeRestart(ctx context.Context, dev *model.Dev, sn string) error {
	log.Infof("restarting services")
	client, _, err := k8Client.GetLocalWithContext(dev.Context)
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
