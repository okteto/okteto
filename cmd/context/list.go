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

package context

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var output string

// Lists all contexts managed by okteto
func List() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#list"),
		Short:   "List available contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := executeListContext(ctx); err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: json|yaml")
	return cmd
}

func executeListContext(ctx context.Context) error {
	contexts := getOktetoClusters(false)
	contexts = append(contexts, getK8sClusters(getKubernetesContextList(true))...)

	if len(contexts) == 0 {
		return fmt.Errorf("no contexts are available. Run 'okteto context' to configure your first okteto context")
	}

	ctxStore := okteto.ContextStore()

	var ctxs []okteto.OktetoContextViewer
	for _, ctxSelector := range contexts {
		okCtx := ctxStore.Contexts[ctxSelector.Name]

		ctxViewer := okteto.OktetoContextViewer{
			Name:     ctxSelector.Name,
			Builder:  "docker",
			Registry: "-",
			Current:  okteto.Context().Name == ctxSelector.Name,
		}
		if okCtx.IsOkteto {
			ctxViewer.Registry = okCtx.Registry
			ctxViewer.Namespace = okCtx.Namespace
			if okCtx.Builder != "" {
				ctxViewer.Builder = okCtx.Builder
			}
			ctxViewer.Managed = true
		} else {
			ctxViewer.Namespace = getKubernetesContextNamespace(ctxSelector.Name)
			ctxViewer.Managed = false
		}
		ctxs = append(ctxs, ctxViewer)
	}

	if output == "" {
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
		fmt.Fprintf(w, "Name\tNamespace\tBuilder\tRegistry\tOkteto Managed\n")
		for _, ctx := range ctxs {
			if ctx.Name == ctxStore.CurrentContext {
				ctx.Name += " *"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n", ctx.Name, ctx.Namespace, ctx.Builder, ctx.Registry, ctx.Managed)
		}
		w.Flush()
	} else if output == "json" {
		ctxRaw, err := json.MarshalIndent(ctxs, "", "\t")
		if err != nil {
			return err
		}
		fmt.Println(string(ctxRaw))
	} else if output == "yaml" {
		ctxRaw, err := yaml.Marshal(&ctxs)
		if err != nil {
			return err
		}
		fmt.Print(string(ctxRaw))
	} else {
		return fmt.Errorf("unable to match a printer suitable for the output format \"%s\", allowed formats are: json, yaml", output)
	}

	return nil
}
