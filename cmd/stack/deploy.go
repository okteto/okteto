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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

//Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	var stackPath string
	var name string
	var namespace string
	var forceBuild bool
	var wait bool
	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: fmt.Sprintf("Deploys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := utils.LoadStack(name, stackPath)
			if err != nil {
				return err
			}

			if err := s.UpdateNamespace(namespace); err != nil {
				return err
			}

			if err := login.WithEnvVar(ctx); err != nil {
				return err
			}

			err = stack.Deploy(ctx, s, forceBuild, wait)
			analytics.TrackDeployStack(err == nil)
			if err == nil {
				log.Success("Successfully deployed stack '%s'", s.Name)
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&stackPath, "file", "f", utils.DefaultStackManifest, "path to the stack manifest file")
	cmd.Flags().StringVarP(&name, "name", "", "", "overwrites the stack name")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is deployed")
	cmd.Flags().BoolVarP(&forceBuild, "build", "", false, "build images before starting any Stack service")
	cmd.Flags().BoolVarP(&wait, "wait", "", false, "wait until a minimum number of containers are in a ready state for every service")
	return cmd
}
