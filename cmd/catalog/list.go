// Copyright 2025 The Okteto Authors
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

package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// errInvalidListOutput is returned when the -o flag is set to an unsupported value.
var errInvalidListOutput = fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'yaml']")

type listFlags struct {
	output     string
	k8sContext string
}

// catalogListOutput is the view model for a single catalog item in list output.
type catalogListOutput struct {
	Name          string   `json:"name" yaml:"name"`
	RepositoryURL string   `json:"repositoryUrl" yaml:"repositoryUrl"`
	Branch        string   `json:"branch" yaml:"branch"`
	ManifestPath  string   `json:"manifestPath" yaml:"manifestPath"`
	Variables     []string `json:"variables" yaml:"variables"`
	ReadOnly      bool     `json:"readOnly" yaml:"readOnly"`
}

// List returns the `okteto catalog list` cobra command.
func List(ctx context.Context) *cobra.Command {
	flags := &listFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the available Okteto Catalog items",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateListOutput(flags.output); err != nil {
				return err
			}

			ctxOptions := &contextCMD.Options{Context: flags.k8sContext}
			if flags.output == "" {
				ctxOptions.Show = true
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}
			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			c, err := NewCommand()
			if err != nil {
				return err
			}
			items, err := c.okClient.Catalog().List(ctx)
			if err != nil {
				var uErr oktetoErrors.UserError
				if errors.As(err, &uErr) {
					return uErr
				}
				return fmt.Errorf("failed to list catalog items: %w", err)
			}
			return displayCatalogItems(os.Stdout, items, flags.output)
		},
	}

	cmd.Flags().StringVarP(&flags.k8sContext, "context", "c", "", "overwrite the current Okteto Context")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format. One of: ['json', 'yaml']")
	return cmd
}

// displayCatalogItems renders the catalog items in the requested format.
func displayCatalogItems(w io.Writer, items []types.GitCatalogItem, format string) error {
	view := toCatalogListOutput(items)
	switch format {
	case "json":
		if len(view) == 0 {
			fmt.Fprintln(w, "[]")
			return nil
		}
		bytes, err := json.MarshalIndent(view, "", " ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(bytes))
	case "yaml":
		bytes, err := yaml.Marshal(view)
		if err != nil {
			return err
		}
		fmt.Fprint(w, string(bytes))
	default:
		if len(view) == 0 {
			fmt.Fprintln(w, "There are no catalog items available")
			return nil
		}
		tw := tabwriter.NewWriter(w, 1, 1, 2, ' ', 0)
		fmt.Fprint(tw, "Name\tRepository\tBranch\tManifest\tRead-Only\n")
		for _, item := range view {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%v\n",
				item.Name,
				item.RepositoryURL,
				valueOrDash(item.Branch),
				valueOrDash(item.ManifestPath),
				item.ReadOnly,
			)
		}
		return tw.Flush()
	}
	return nil
}

// toCatalogListOutput maps API types into stable display structs, sorted by name
// so output is deterministic across runs.
func toCatalogListOutput(items []types.GitCatalogItem) []catalogListOutput {
	out := make([]catalogListOutput, 0, len(items))
	for _, it := range items {
		vars := make([]string, 0, len(it.Variables))
		for _, v := range it.Variables {
			vars = append(vars, v.Name)
		}
		out = append(out, catalogListOutput{
			Name:          it.Name,
			RepositoryURL: it.RepositoryURL,
			Branch:        it.Branch,
			ManifestPath:  it.ManifestPath,
			Variables:     vars,
			ReadOnly:      it.ReadOnly,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func validateListOutput(output string) error {
	switch output {
	case "", "json", "yaml":
		return nil
	default:
		return errInvalidListOutput
	}
}
