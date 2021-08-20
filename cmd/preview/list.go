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

package preview

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// List lists all the previews
func List(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all preview environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := executeListPreviews(ctx)
			return err

		},
	}

	return cmd
}

func executeListPreviews(ctx context.Context) error {
	previewList, err := okteto.ListPreviews(ctx)
	if err != nil {
		return fmt.Errorf("failed to get preview environments: %s", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tScope\tSleeping\n")
	for _, preview := range previewList {
		fmt.Fprintf(w, "%s\t%s\t%v\n", preview.ID, preview.Scope, preview.Sleeping)
	}

	w.Flush()
	return nil
}
