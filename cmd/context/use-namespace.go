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

package context

import (
	"context"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// UseNamespace changes your current context namespace.
func UseNamespace() *cobra.Command {
	ctxOptions := &Options{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "use-namespace [name]",
		Args:   utils.ExactArgsAccepted(1, "https://okteto.com/docs/reference/cli/#use-1"),
		Short:  "Set the namespace of the okteto context",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto context use-namespace' is deprecated in favor of 'okteto namespace', and will be removed in a future version")
			ctx := context.Background()
			ctxOptions.Namespace = args[0]
			ctxOptions.Context = okteto.GetContext().Name
			ctxOptions.Show = false
			ctxOptions.Save = true
			ctxOptions.IsCtxCommand = true

			err := NewContextCommand().Run(ctx, ctxOptions)
			analytics.TrackContextUseNamespace(err == nil)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
