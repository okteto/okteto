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

package cmd

import (
	"context"

	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/spf13/cobra"
)

// Create creates resources
func Create(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "create",
		Short:  "Create resources",
		Args:   utils.NoArgsAccepted(""),
	}
	cmd.AddCommand(deprecatedCreateNamespace(ctx))
	return cmd
}

func deprecatedCreateNamespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace <name>",
		Short: "Create a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.RunE(namespace.Create(ctx), args)
		},
		Args: utils.ExactArgsAccepted(1, ""),
	}
	return cmd
}
