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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	errInvalidOutput = errors.New("output format is not accepted. Value must be one of: ['json', 'yaml']")
)

type listFlags struct {
	output string
}

type namespaceOutput struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Status    string `json:"status" yaml:"status"`
	Current   bool   `json:"current" yaml:"current"`
}

// List all namespace in current context
func List(ctx context.Context, ioCtrl *io.Controller) *cobra.Command {
	flags := &listFlags{}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List your Okteto Namespaces",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			nsCmd, err := NewCommand(ioCtrl)
			if err != nil {
				return err
			}
			err = nsCmd.executeListNamespaces(ctx, flags.output)
			return err
		},
		Args: utils.NoArgsAccepted(""),
	}

	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format. One of: ['json', 'yaml']")
	return cmd
}

func (nc *Command) executeListNamespaces(ctx context.Context, output string) error {
	if err := validateNamespaceListOutput(output); err != nil {
		return err
	}

	spaces, err := nc.okClient.Namespaces().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}

	namespaces := getNamespaceOutput(spaces)
	return nc.displayListNamespaces(namespaces, output)
}

func (nc *Command) displayListNamespaces(namespaces []namespaceOutput, output string) error {
	switch output {
	case "json":
		if len(namespaces) == 0 {
			nc.ioCtrl.Out().Println("[]")
			return nil
		}
		b, err := json.MarshalIndent(namespaces, "", " ")
		if err != nil {
			return err
		}
		nc.ioCtrl.Out().Println(string(b))
	case "yaml":
		b, err := yaml.Marshal(namespaces)
		if err != nil {
			return err
		}
		nc.ioCtrl.Out().Print(string(b))
	default:
		if len(namespaces) == 0 {
			nc.ioCtrl.Out().Println("There are no namespaces")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
		fmt.Fprintf(w, "Namespace\tStatus\n")
		for _, space := range namespaces {
			id := space.Namespace
			if id == okteto.GetContext().Namespace {
				id += " *"
			}
			fmt.Fprintf(w, "%s\t%v\n", id, space.Status)
		}
		w.Flush()
	}
	return nil
}

func validateNamespaceListOutput(output string) error {
	switch output {
	case "", "json", "yaml":
		return nil
	default:
		return errInvalidOutput
	}
}

// getNamespaceOutput transforms type.Namespace into namespaceOutput type
func getNamespaceOutput(namespaces []types.Namespace) []namespaceOutput {
	var namespaceSlice []namespaceOutput
	currentNamespace := okteto.GetContext().Namespace
	for _, ns := range namespaces {
		previewOutput := namespaceOutput{
			Namespace: ns.ID,
			Status:    ns.Status,
			Current:   ns.ID == currentNamespace,
		}
		namespaceSlice = append(namespaceSlice, previewOutput)
	}
	return namespaceSlice
}
