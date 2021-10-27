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
	"sort"
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
		Short:   "Lists okteto contexts",
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
	oCtxs := okteto.ContextStore()
	contexts := make([]string, 0)
	for name := range oCtxs.Contexts {
		contexts = append(contexts, name)

	}
	sort.Slice(contexts, func(i, j int) bool {
		return len(contexts[i]) < len(contexts[j])
	})

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tNamespace\tBuilder\tRegistry\n")
	for _, okctxName := range contexts {
		okCtx := oCtxs.Contexts[okctxName]
		name := okteto.RemoveSchema(okCtx.Name)
		if okCtx.Name == oCtxs.CurrentContext {
			name += " *"
		}
		builder := "docker"
		if okCtx.Buildkit != "" {
			builder = okCtx.Buildkit
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, okCtx.Namespace, builder, okCtx.Registry)
	}

	w.Flush()
	return nil
}
