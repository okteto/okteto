// Copyright 2022 The Okteto Authors
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

package cmd

import (
	"context"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

// List lists resources
func List(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List resources",
		Args:   utils.NoArgsAccepted(""),
	}
	cmd.AddCommand(deprecatedListNamespace(ctx))
	return cmd
}

func deprecatedListNamespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace <name>",
		Short: "List namespaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto list namespace' is deprecated in favor of 'okteto namespace list', and will be removed in version 1.16")
			return cmd.RunE(namespace.List(ctx), args)
		},
		Args: utils.NoArgsAccepted(""),
	}
	return cmd
}
