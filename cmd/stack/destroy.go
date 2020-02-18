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
	"github.com/spf13/cobra"
)

//Destroy destroys a stack
func Destroy(ctx context.Context) *cobra.Command {
	var stackPath string
	var namespace string
	var rm bool
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: fmt.Sprintf("Destroys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := executeDestroyStack(ctx)
			analytics.TrackDestroyStack(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&stackPath, "file", "f", "okteto-stack.yaml", "path to the stack manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the destroy command is executed")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volumes")
	return cmd
}

func executeDestroyStack(ctx context.Context) error {
	return nil
}
