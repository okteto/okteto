// Copyright 2020 The Okteto Authors
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
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// List all namespace in current context
func List(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "namespace",
		Short: "List namespaces managed by Okteto in your current context",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := executeListNamespaces(ctx)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}

func executeListNamespaces(ctx context.Context) error {
	spaces, err := okteto.ListNamespaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %s", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
	fmt.Fprintf(w, "Namespace\tisActive\n")
	for _, space := range spaces {
		fmt.Fprintf(w, "%s\t%v\n", space.ID, !space.Sleeping)
	}

	w.Flush()
	return nil
}
