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
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Endpoints show all the endpoints of a preview environment
func Endpoints(ctx context.Context) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "endpoints <name>",
		Short: "Show endpoints for a preview environment",
		Args:  utils.ExactArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}
			previewName := args[0]
			err := executeListPreviewEndpoints(ctx, previewName)
			return err
		},
	}

	return cmd
}

func executeListPreviewEndpoints(ctx context.Context, name string) error {
	previewList, err := okteto.ListPreviewsEndpoints(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get preview environments: %s", err)
	}

	if len(previewList) == 0 {
		fmt.Printf("There are no available endpoints for preview '%s'\n", name)
	} else {
		fmt.Printf("Available endpoints for preview '%s'\n  - %s\n", name, strings.Join(previewList, "\n  - "))
	}
	return nil
}
