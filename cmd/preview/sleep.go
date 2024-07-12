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

package preview

import (
	"context"
	"fmt"
	"github.com/okteto/okteto/pkg/vars"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Sleep sleeps a preview environment
func Sleep(ctx context.Context, varManager *vars.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sleep <name>",
		Short: "Sleeps a preview environment",
		Args:  utils.ExactArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			prToSleep := args[0]

			if err := contextCMD.NewContextCommand(contextCMD.WithVarManager(varManager)).Run(ctx, &contextCMD.Options{Show: true}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			prCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = prCmd.ExecuteSleepPreview(ctx, prToSleep)
			return err
		},
	}
	return cmd
}

func (pr *Command) ExecuteSleepPreview(ctx context.Context, preview string) error {
	// Spinner to be loaded sleeping a preview environment
	oktetoLog.Spinner(fmt.Sprintf("Sleeping preview environment '%s'...", preview))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	// trigger preview environment to sleep
	if err := pr.okClient.Namespaces().Sleep(ctx, preview); err != nil {
		return fmt.Errorf("%w: %w", errFailedSleepPreview, err)
	}

	oktetoLog.Success("Preview environment '%s' is sleeping", preview)
	return nil
}
