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
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Lists all contexts managed by okteto
func List() *cobra.Command {
	var outputFormat = "plain"
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#list"),
		Short:   "List available contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := executeListContext(ctx, outputFormat); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "plain", "Output format. One of: json|yaml|plain.")
	return cmd
}

func executeListContext(ctx context.Context, outputFormat string) error {
	contexts := getOktetoClusters(false)
	contexts = append(contexts, getK8sClusters(getKubernetesContextList(true))...)

	if len(contexts) == 0 {
		return fmt.Errorf("no contexts are available. Run 'okteto context' to configure your first okteto context")
	}

	switch outputFormat {
	case "json":
		return jsonOutput(contexts)
	case "yaml":
		return yamlOutput(contexts)
	case "plain":
		plainOutput(contexts)
		return nil
	default:
		return fmt.Errorf("unknown format")
	}
}

func plainOutput(contexts []utils.SelectorItem) {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tNamespace\tBuilder\tRegistry\n")
	ctxStore := okteto.ContextStore()
	for _, ctxSelector := range contexts {
		if okCtx, ok := ctxStore.Contexts[ctxSelector.Name]; ok && okCtx.Builder != "" {
			ctxSelector.Builder = okCtx.Builder
		}

		if ctxSelector.Name == ctxStore.CurrentContext {
			ctxSelector.Name += " *"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ctxSelector.Name, ctxSelector.Namespace, ctxSelector.Builder, ctxSelector.Registry)
	}

	w.Flush()
}

func jsonOutput(contexts []utils.SelectorItem) error {
	b, err := json.MarshalIndent(contexts, "", "  ")
	if err != nil {
		return err
	}

	fmt.Print(string(b))
	return nil
}

func yamlOutput(contexts []utils.SelectorItem) error {
	b, err := yaml.Marshal(contexts)
	if err != nil {
		return err
	}

	fmt.Print(string(b))
	return nil
}
