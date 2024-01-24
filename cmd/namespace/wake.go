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

package namespace

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func Wake(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wake <name>",
		Short: "Wakes a namespace",
		Args:  utils.MaximumNArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			nsToWake := okteto.GetContext().Namespace
			if len(args) > 0 {
				nsToWake = args[0]
			}
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Namespace: nsToWake, Show: true}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}
			nsCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.ExecuteWakeNamespace(ctx, nsToWake)
			return err
		},
	}
	return cmd
}

func (nc *Command) ExecuteWakeNamespace(ctx context.Context, namespace string) error {
	// Spinner to be loaded before waking a namespace
	oktetoLog.Spinner(fmt.Sprintf("Waking %s namespace", namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	// trigger namespace to sleep
	if err := nc.okClient.Namespaces().Wake(ctx, namespace); err != nil {
		return fmt.Errorf("%w: %w", errFailedWakeNamespace, err)
	}

	oktetoLog.Success("Namespace '%s' is awake now", namespace)
	return nil
}
