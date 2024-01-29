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

package cmd

import (
	"context"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Delete creates resources
func Delete(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "delete",
		Short:  "Delete resources",
		Args:   utils.NoArgsAccepted(""),
	}
	cmd.AddCommand(deprecatedDeleteNamespace(ctx))
	return cmd
}

func deprecatedDeleteNamespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace <name>",
		Short: "Delete a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto delete namespace' is deprecated in favor of 'okteto namespace delete', and will be removed in a future version")
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return errors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := namespace.NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.ExecuteDeleteNamespace(ctx, args[0], nil)
			analytics.TrackDeleteNamespace(err == nil)
			return err
		},
		Args: utils.ExactArgsAccepted(1, ""),
	}
	return cmd
}
