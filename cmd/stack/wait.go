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

	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/spf13/cobra"
)

//wait waits for a stack service
func Wait(ctx context.Context) *cobra.Command {
	var stackName string
	var namespace string
	var svcName string
	cmd := &cobra.Command{
		Use:    "wait <name>",
		Short:  "Waits for a stack service",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if namespace != "" {
				namespace = client.GetContextNamespace("")
			}
			if stackName == "" {
				return fmt.Errorf("Invalid command: stack name must is required.")
			}
			if svcName == "" {
				return fmt.Errorf("Invalid command: service name is required.")
			}

			if err := stack.Wait(ctx, stackName, svcName, namespace); err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&stackName, "stack", "", "", "stack name")
	cmd.Flags().StringVarP(&svcName, "service", "", "", "service name that must wait")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "the namespace where the stack is deployed")
	return cmd
}
