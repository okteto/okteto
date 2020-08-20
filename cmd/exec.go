// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"

	k8Client "github.com/okteto/okteto/pkg/k8s/client"

	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
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

			dev, err := utils.LoadDev(devPath)
			if err != nil {
				return err
			}
			dev.UpdateContext(namespace, k8sContext)
			err = executeExec(ctx, dev, args)
			analytics.TrackExec(err == nil)

			if errors.IsNotFound(err) {
				return errors.UserError{
					E:    fmt.Errorf("Development container not found in namespace %s", dev.Namespace),
					Hint: "Run `okteto up` to launch it or use `okteto namespace` to select the correct namespace and try again",
				}
			}

			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("exec requires the COMMAND argument")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the exec command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the exec command is executed")

	return cmd
}

func executeExec(ctx context.Context, dev *model.Dev, args []string) error {

	wrapped := []string{"sh", "-c"}
	wrapped = append(wrapped, args...)

	if dev.ExecuteOverSSHEnabled() || dev.RemoteModeEnabled() {
		log.Infof("executing remote command over SSH")
		return ssh.Exec(ctx, dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, wrapped)
	}

	client, cfg, namespace, err := k8Client.GetLocal(dev.Context)
	if err != nil {
		return err
	}

	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	p, err := pods.GetDevPod(ctx, dev, client, false)
	if err != nil {
		return err
	}

	if dev.Container == "" {
		dev.Container = p.Spec.Containers[0].Name
	}

	return exec.Exec(ctx, client, cfg, dev.Namespace, p.Name, dev.Container, true, os.Stdin, os.Stdout, os.Stderr, wrapped)
}
