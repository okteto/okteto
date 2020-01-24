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

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/model"

	k8Client "github.com/okteto/okteto/pkg/k8s/client"

	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	var devPath string
	var namespace string

	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Execute a command in your development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}
			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}
			err = executeExec(ctx, dev, args)
			analytics.TrackExec(err == nil)

			if errors.IsNotFound(err) {
				return errors.UserError{
					E:    fmt.Errorf("Development environment not found in namespace %s", dev.Namespace),
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

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the exec command is executed")

	return cmd
}

func executeExec(ctx context.Context, dev *model.Dev, args []string) error {
	client, cfg, namespace, err := k8Client.GetLocal()
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

	if len(dev.Container) == 0 {
		dev.Container = p.Spec.Containers[0].Name
	}

	return exec.Exec(ctx, client, cfg, dev.Namespace, p.Name, dev.Container, true, os.Stdin, os.Stdout, os.Stderr, args)
}
