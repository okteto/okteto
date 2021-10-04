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
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/status"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"

	"github.com/spf13/cobra"
)

// Exec executes a command on the CND container
func Exec() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string

	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Execute a command in your development container",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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

			t := time.NewTicker(1 * time.Second)
			iter := 0
			err = executeExec(ctx, dev, args)
			for errors.IsTransient(err) {
				if iter == 0 {
					log.Yellow("Connection lost to your development container, reconnecting...")
				}
				iter++
				iter = iter % 10
				<-t.C
				err = executeExec(ctx, dev, args)
			}

			analytics.TrackExec(err == nil)

			if errors.IsNotFound(err) {
				return errors.UserError{
					E:    fmt.Errorf("Development container not found in namespace %s", dev.Namespace),
					Hint: "Run 'okteto up' to launch it or use 'okteto namespace' to select the correct namespace and try again",
				}
			}

			return err
		},
		Args: utils.MinimumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#exec"),
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the exec command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the exec command is executed")

	return cmd
}

func executeExec(ctx context.Context, dev *model.Dev, args []string) error {

	wrapped := []string{"sh", "-c"}
	wrapped = append(wrapped, args...)

	c, cfg, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		return err
	}

	pod, err := apps.GetRunningPodInLoop(ctx, dev, app, c)
	if err != nil {
		return err
	}

	if pod == nil {
		return errors.UserError{
			E:    fmt.Errorf("development mode is not enabled"),
			Hint: "Run 'okteto up' to enable it and try again",
		}
	}

	waitForStates := []config.UpState{config.Ready}
	if err := status.Wait(ctx, dev, waitForStates); err != nil {
		return err
	}

	if dev.Container == "" {
		dev.Container = pod.Spec.Containers[0].Name
	}

	if dev.RemoteModeEnabled() {
		if dev.RemotePort == 0 {
			p, err := ssh.GetPort(dev.Name)
			if err != nil {
				log.Infof("failed to get the SSH port for %s: %s", dev.Name, err)
				return errors.UserError{
					E:    fmt.Errorf("development mode is not enabled on your deployment"),
					Hint: "Run 'okteto up' to enable it and try again",
				}
			}

			dev.RemotePort = p
			log.Infof("executing remote command over SSH port %d", dev.RemotePort)
		}

		dev.LoadRemote(ssh.GetPublicKey())

		return ssh.Exec(ctx, dev.Interface, dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, wrapped)
	}

	return exec.Exec(ctx, c, cfg, dev.Namespace, pod.Name, dev.Container, true, os.Stdin, os.Stdout, os.Stderr, wrapped)
}
