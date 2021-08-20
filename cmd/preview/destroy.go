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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Destroy destroy a preview
func Destroy(ctx context.Context) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "destroy <name>",
		Short: "Destroy a preview environment",
		Args:  utils.ExactArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			name = getExpandedName(args[0])
			err := executeDestroyPreview(ctx, name)
			analytics.TrackPreviewDestroy(err == nil)
			return err
		},
	}

	return cmd
}

func executeDestroyPreview(ctx context.Context, name string) error {
	if err := okteto.DestroyPreview(ctx, name); err != nil {
		return fmt.Errorf("failed to delete namespace: %s", err)
	}

	log.Success("Preview environment destroyed")
	return nil
}
