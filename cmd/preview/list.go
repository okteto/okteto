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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// listFlags are the flags available for list commands
type listFlags struct {
	labels []string
	output string
}

type previewOutput struct {
	Name     string   `json:"name" yaml:"name"`
	Scope    string   `json:"scope" yaml:"scope"`
	Sleeping bool     `json:"sleeping" yaml:"sleeping"`
	Labels   []string `json:"labels" yaml:"labels"`
}

// List lists all the previews
func List(ctx context.Context) *cobra.Command {
	flags := &listFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all preview environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxOptions := &contextCMD.ContextOptions{}

			if flags.output == "" {
				ctxOptions.Show = true
			}

			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			if err := validatePreviewListOutput(flags.output); err != nil {
				return err
			}

			oktetoClient, err := okteto.NewOktetoClient()
			if err != nil {
				return err
			}

			previewList, err := oktetoClient.Previews().List(ctx, flags.labels)
			if err != nil {
				if uErr, ok := err.(oktetoErrors.UserError); ok {
					return uErr
				}
				return fmt.Errorf("failed to get preview environments: %s", err)
			}

			previewOutput := getPreviewOutput(previewList)

			return executeListPreviews(previewOutput, flags.output)
		},
	}
	cmd.Flags().StringArrayVarP(&flags.labels, "label", "", []string{}, "tag and organize preview environments using labels (multiple --label flags accepted)")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format. One of: ['json', 'yaml']")

	return cmd
}

// executeListPreviews prints the list of previews
func executeListPreviews(previews []previewOutput, outputFormat string) error {
	if len(previews) == 0 {
		fmt.Println("There are no previews")
		return nil
	}
	switch outputFormat {
	case "json":
		bytes, err := json.MarshalIndent(previews, "", " ")
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	case "yaml":
		bytes, err := yaml.Marshal(previews)
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	default:
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
		fmt.Fprint(w, "Name\tScope\tSleeping\tLabels\n")
		for _, preview := range previews {
			output := getPreviewDefaultOutput(preview)
			fmt.Fprint(w, output)
		}
		w.Flush()
	}
	return nil
}

// getPreviewDefaultOutput returns the rows for the default list output format
func getPreviewDefaultOutput(preview previewOutput) string {
	previewLabels := "-"
	if len(preview.Labels) > 0 {
		previewLabels = strings.Join(preview.Labels, ", ")
	}
	return fmt.Sprintf("%s\t%s\t%v\t%s\n", preview.Name, preview.Scope, preview.Sleeping, previewLabels)
}

// getPreviewOutput transforms type.Preview into previewOutput type
func getPreviewOutput(previews []types.Preview) []previewOutput {
	var previewSlice []previewOutput
	for _, p := range previews {
		previewOutput := previewOutput{
			Name:     p.ID,
			Scope:    p.Scope,
			Sleeping: p.Sleeping,
			Labels:   p.PreviewLabels,
		}
		previewSlice = append(previewSlice, previewOutput)
	}
	return previewSlice
}

// validatePreviewListOutput returns error if output flag is not valid
func validatePreviewListOutput(output string) error {
	switch output {
	case "", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'yaml']")
	}
}
