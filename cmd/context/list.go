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

package context

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Lists all contexts managed by okteto
func List() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#context"),
		Short:   "List available contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := executeListContext(ctx); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func executeListContext(ctx context.Context) error {
	contexts := getOktetoClusters(false)
	contexts = append(contexts, getK8sClusters(getKubernetesContextList(true))...)

	if len(contexts) == 0 {
		return fmt.Errorf("no contexts are available. Run 'okteto context' to configure your first okteto context")
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tNamespace\tBuilder\tRegistry\n")
	ctxStore := okteto.ContextStore()
	for _, ctxSelector := range contexts {
		if okCtx, ok := ctxStore.Contexts[ctxSelector.Name]; ok && okCtx.Builder != "" {
			ctxSelector.Builder = okCtx.Builder
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ctxSelector.Name, ctxSelector.Namespace, ctxSelector.Builder, ctxSelector.Registry)
	}

	w.Flush()
	return nil
}
