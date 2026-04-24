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

package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// infoOptions holds the CLI flags for sandbox info.
type infoOptions struct {
	Namespace  string
	K8sContext string
}

// Info returns the cobra command for "okteto sandbox info <deployment>".
func Info(ctx context.Context) *cobra.Command {
	opts := &infoOptions{}

	cmd := &cobra.Command{
		Use:          "info <deployment>",
		Short:        "Show the status of an active sandbox session",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			ctxOpts := &contextCMD.Options{
				Show:      true,
				Context:   opts.K8sContext,
				Namespace: opts.Namespace,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			ns := okteto.GetContext().Namespace

			statePath := filepath.Join(config.GetOktetoHome(), ns, name, "okteto.state")
			if _, err := os.Stat(statePath); os.IsNotExist(err) {
				oktetoLog.Println(fmt.Sprintf("Sandbox %q is not running", name))
				return oktetoErrors.UserError{
					E: fmt.Errorf("sandbox %q is not running", name),
				}
			}

			state, _ := config.GetState(name, ns)
			return printState(name, state)
		},
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "the namespace to use (defaults to the current okteto context namespace)")
	cmd.Flags().StringVarP(&opts.K8sContext, "context", "c", "", "the name of the okteto context to use (defaults to the current one)")

	return cmd
}

func printState(name string, state config.UpState) error {
	switch state {
	case config.Activating:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is starting (activating dev container...)", name))
	case config.Starting:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is starting (scheduling pod...)", name))
	case config.Attaching:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is starting (attaching persistent volume...)", name))
	case config.Pulling:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is starting (pulling image...)", name))
	case config.StartingSync:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is starting (initialising file sync...)", name))
	case config.Synchronizing:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is running and syncing files", name))
	case config.Ready:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is running", name))
	case config.Failed:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q has failed", name))
		return oktetoErrors.UserError{
			E: fmt.Errorf("sandbox %q has failed", name),
		}
	default:
		oktetoLog.Println(fmt.Sprintf("Sandbox %q is in an unknown state (%s)", name, state))
	}
	return nil
}
