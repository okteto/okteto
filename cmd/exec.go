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
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/status"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/exec"
	oktetoLog "github.com/okteto/okteto/pkg/log"
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

			manifestOpts := contextCMD.ManifestOptions{Filename: devPath, Namespace: namespace, K8sContext: k8sContext}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts)
			if err != nil {
				return err
			}

			dev, err := utils.GetDevFromManifest(manifest, "")
			if err != nil {
				return err
			}

			t := time.NewTicker(1 * time.Second)
			iter := 0
			err = executeExec(ctx, dev, args)
			for oktetoErrors.IsTransient(err) {
				if iter == 0 {
					oktetoLog.Yellow("Connection lost to your development container, reconnecting...")
				}
				iter++
				iter = iter % 10
				<-t.C
				err = executeExec(ctx, dev, args)
			}

			analytics.TrackExec(err == nil)

			if oktetoErrors.IsNotFound(err) {
				return oktetoErrors.UserError{
					E:    fmt.Errorf("development container not found in namespace '%s'", dev.Namespace),
					Hint: "Run 'okteto up' to launch your development container or use 'okteto context' to change your current context",
				}
			}

			return err
		},
		Args: utils.MinimumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#exec"),
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
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

	devName := dev.Name
	var devApp apps.App
	if !dev.Autocreate {
		app, err := apps.Get(ctx, dev, dev.Namespace, c)
		if err != nil {
			return err
		}

		retries := 0
		ticker := time.NewTicker(500 * time.Millisecond)
		for {
			if apps.IsDevModeOn(app) {
				break
			}
			retries++
			if retries >= 10 {
				return oktetoErrors.UserError{
					E:    fmt.Errorf("development mode is not enabled"),
					Hint: "Run 'okteto up' to enable it and try again",
				}
			}
			<-ticker.C
		}
		waitForStates := []config.UpState{config.Ready}
		if err := status.Wait(ctx, dev, waitForStates); err != nil {
			return err
		}

		devApp = app.DevClone()
	} else {
		dev.Name = model.DevCloneName(dev.Name)
		devApp, err = apps.Get(ctx, dev, dev.Namespace, c)
		if err != nil {
			return err
		}
	}

	if err := devApp.Refresh(ctx, c); err != nil {
		return err
	}
	pod, err := devApp.GetRunningPod(ctx, c)
	if err != nil {
		return err
	}

	if dev.Container == "" {
		dev.Container = pod.Spec.Containers[0].Name
	}

	if dev.RemoteModeEnabled() {
		p, err := ssh.GetPort(devName)
		if err != nil {
			oktetoLog.Infof("failed to get the SSH port for %s: %s", devName, err)
			return oktetoErrors.UserError{
				E:    fmt.Errorf("development mode is not enabled on your deployment"),
				Hint: "Run 'okteto up' to enable it and try again",
			}
		}

		dev.RemotePort = p
		oktetoLog.Infof("executing remote command over SSH port %d", dev.RemotePort)

		dev.LoadRemote(ssh.GetPublicKey())

		return ssh.Exec(ctx, dev.Interface, dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, wrapped)
	}

	return exec.Exec(ctx, c, cfg, dev.Namespace, pod.Name, dev.Container, true, os.Stdin, os.Stdout, os.Stderr, wrapped)
}
