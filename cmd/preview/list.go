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
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// ListFlags are the flags available for list commands
type ListFlags struct {
	labels []string
}

// List lists all the previews
func List(ctx context.Context) *cobra.Command {
	flags := &ListFlags{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all preview environments",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{
				Show: true,
			}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			err := executeListPreviews(ctx, *flags)
			return err

		},
	}
	cmd.Flags().StringArrayVarP(&flags.labels, "label", "", []string{}, "set a preview environment label (can be set more than once)")

	return cmd
}

func executeListPreviews(ctx context.Context, opts ListFlags) error {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	previewList, err := oktetoClient.Previews().List(ctx, opts.labels)
	if err != nil {
		if uErr, ok := err.(oktetoErrors.UserError); ok {
			return uErr
		}
		return fmt.Errorf("failed to get preview environments: %s", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tScope\tSleeping\tLabels\n")
	for _, preview := range previewList {
		previewLabels := "-"
		if len(preview.PreviewLabels) > 0 {
			previewLabels = strings.Join(preview.PreviewLabels, ", ")
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", preview.ID, preview.Scope, preview.Sleeping, previewLabels)
	}

	w.Flush()
	return nil
}
