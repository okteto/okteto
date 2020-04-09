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

package stack

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

//Destroy destroys a stack
func Destroy(ctx context.Context) *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "destroy <name>",
		Short: fmt.Sprintf("Destroys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			err := stack.Destroy(ctx, name, namespace)
			analytics.TrackDestroyStack(err == nil)
			if err == nil {
				log.Success("Successfully destroyed stack '%s'", name)
			}
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("destroy requires the stack NAME argument")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is destroyed")
	return cmd
}
