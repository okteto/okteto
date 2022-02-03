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

package namespace

import (
	"context"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

//NamespaceCommand has all the namespaces subcommands
type NamespaceCommand struct {
	ctxCmd   *contextCMD.ContextCommand
	okClient types.OktetoInterface
}

//NewCommand creates a namespace command to
func NewCommand() (*NamespaceCommand, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	return &NamespaceCommand{
		ctxCmd:   contextCMD.NewContextCommand(),
		okClient: c,
	}, nil
}

// Namespace fetch credentials for a cluster namespace
func Namespace(ctx context.Context) *cobra.Command {
	options := &UseOptions{}
	cmd := &cobra.Command{
		Use:     "namespace [name]",
		Short:   "Configure the current namespace of the okteto context",
		Aliases: []string{"ns"},
		Args:    utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {

			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}
			if options.personal {
				namespace = okteto.Context().PersonalNamespace
			}
			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.Use(ctx, namespace)
			if err != nil {
				return err
			}

			analytics.TrackNamespace(err == nil, len(args) > 0)
			return err
		},
	}
	cmd.Flags().BoolVarP(&options.personal, "personal", "", false, "Load personal account")

	cmd.AddCommand(Use(ctx))
	cmd.AddCommand(List(ctx))
	cmd.AddCommand(Create(ctx))
	cmd.AddCommand(Delete(ctx))
	return cmd
}
