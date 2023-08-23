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

// ListFlags are the flags available for list commands
type ListFlags struct {
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
	flags := &ListFlags{}
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

			if err := ValidateOutput(flags.output); err != nil {
				return err
			}

			err := executeListPreviews(ctx, *flags)
			return err
		},
	}
	cmd.Flags().StringArrayVarP(&flags.labels, "label", "", []string{}, "tag and organize preview environments using labels (multiple --label flags accepted)")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format. One of: ['json', 'yaml']")

	return cmd
}

func executeListPreviews(ctx context.Context, opts ListFlags) error {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	previewListOutput, err := getPreviewOutput(ctx, opts, oktetoClient)
	if err != nil {
		return err
	}
	switch opts.output {
	case "json":
		bytes, err := json.MarshalIndent(previewListOutput, "", " ")
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	case "yaml":
		bytes, err := yaml.Marshal(previewListOutput)
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	default:
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
		fmt.Fprint(w, "Name\tScope\tSleeping\tLabels\n")
		for _, preview := range previewListOutput {
			output := getPreviewDefaultOutput(preview)
			fmt.Fprint(w, output)
		}
		w.Flush()
	}
	return nil
}

func getPreviewDefaultOutput(preview previewOutput) string {
	previewLabels := "-"
	if len(preview.Labels) > 0 {
		previewLabels = strings.Join(preview.Labels, ", ")
	}
	return fmt.Sprintf("%s\t%s\t%v\t%s\n", preview.Name, preview.Scope, preview.Sleeping, previewLabels)
}

func getPreviewOutput(ctx context.Context, opts ListFlags, oktetoClient types.OktetoInterface) ([]previewOutput, error) {
	var previewSlice []previewOutput
	previewList, err := oktetoClient.Previews().List(ctx, opts.labels)
	if err != nil {
		if uErr, ok := err.(oktetoErrors.UserError); ok {
			return nil, uErr
		}
		return nil, fmt.Errorf("failed to get preview environments: %s", err)
	}
	for _, preview := range previewList {
		previewOutput := previewOutput{
			Name:     preview.ID,
			Scope:    preview.Scope,
			Sleeping: preview.Sleeping,
			Labels:   preview.PreviewLabels,
		}
		previewSlice = append(previewSlice, previewOutput)
	}
	return previewSlice, nil
}

func ValidateOutput(output string) error {
	switch output {
	case "", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'yaml']")
	}
}
