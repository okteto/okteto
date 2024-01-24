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
	"os"
	"text/tabwriter"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// List all namespace in current context
func List(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List namespaces managed by Okteto in your current context",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand()
			if err != nil {
				return err
			}
			err = nsCmd.executeListNamespaces(ctx)
			return err
		},
		Args: utils.NoArgsAccepted(""),
	}
}

func (nc *Command) executeListNamespaces(ctx context.Context) error {
	spaces, err := nc.okClient.Namespaces().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Namespace\tStatus\n")
	for _, space := range spaces {
		if space.ID == okteto.GetContext().Namespace {
			space.ID += " *"
		}
		fmt.Fprintf(w, "%s\t%v\n", space.ID, space.Status)
	}

	w.Flush()
	return nil
}
