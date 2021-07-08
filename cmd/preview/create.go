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
	"errors"
	"fmt"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Create creates a preview environment
func Create(ctx context.Context) *cobra.Command {
	var previewScope string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Creates a preview environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePreviewType(previewScope); err != nil {
				return err
			}

			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := executeCreatePreview(ctx, args[0], previewScope)
			analytics.TrackCreatePreview(err == nil)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("'preview destroy' requires one argument")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&previewScope, "scope", "s", "personal", "the scope of preview environment to create. Accepted values are ['personal', 'global']")

	return cmd
}

func validatePreviewType(previewType string) error {
	if !(previewType == "global" || previewType == "personal") {
		return fmt.Errorf("Value '%s' is invalid for flag 'type'. Accepted values are ['global', 'personal']", previewType)
	}
	return nil
}

func executeCreatePreview(ctx context.Context, name, previewScope string) error {
	oktetoNS, err := okteto.CreatePreview(ctx, name, previewScope)
	if err != nil {
		return err
	}

	log.Success("Preview environment '%s' created", oktetoNS)

	return nil
}
