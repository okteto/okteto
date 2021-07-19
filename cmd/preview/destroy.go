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

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Destroy destroy a preview
func Destroy(ctx context.Context) *cobra.Command {
	var branch string
	var repository string
	var scope string
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy a preview environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := executeDeletePreview(ctx, branch, repository, scope)
			analytics.TrackDeletePreview(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "personal", "the scope of preview environment to create. Accepted values are ['personal', 'global']")

	return cmd
}

func executeDeletePreview(ctx context.Context, branch, repository, scope string) error {
	if err := okteto.DestroyPreview(ctx, branch, repository, scope); err != nil {
		return fmt.Errorf("failed to delete namespace: %s", err)
	}

	log.Success("Preview environment deleted")
	return nil
}
