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

package namespace

import (
	"context"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Namespace fetch credentials for a cluster namespace
func Namespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "namespace [name]",
		Hidden: true,
		Short:  "Download k8s credentials for a namespace",
		Args:   utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {

			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			if !okteto.IsOkteto() {
				return errors.ErrContextIsNotOktetoCluster
			}
			err := contextCMD.Run(
				ctx,
				&contextCMD.ContextOptions{
					Namespace: namespace,
				},
			)

			if err != nil {
				return err
			}

			if err := contextCMD.ExecuteUpdateKubeconfig(ctx); err != nil {
				return err
			}

			analytics.TrackNamespace(err == nil)
			return err
		},
	}
	return cmd
}
