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

package stack

import (
	"context"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Destroy destroys a stack
func Destroy(ctx context.Context) *cobra.Command {
	var stackPath []string
	var name string
	var namespace string
	var rm bool
	cmd := &cobra.Command{
		Use:   "destroy <name>",
		Short: "Destroys a stack",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#destroy-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxResource, err := utils.LoadStackContext(stackPath)
			if err != nil {
				return err
			}

			if err := ctxResource.UpdateNamespace(namespace); err != nil {
				return err
			}

			if err := contextCMD.Init(ctx, ctxResource); err != nil {
				return err
			}

			s, err := utils.LoadStack(name, stackPath)
			if err != nil {
				if name == "" {
					return err
				}
				log.Errorf("error reading stack: %s", err.Error())
				s = &model.Stack{Name: name}
			}
			s.Namespace = okteto.Context().Namespace

			to, err := model.GetTimeout()
			if err != nil {
				return err
			}

			err = stack.Destroy(ctx, s, rm, to)
			analytics.TrackDestroyStack(err == nil)
			if err == nil {
				log.Success("Stack '%s' successfully destroyed", s.Name)
			}
			return err
		},
	}
	cmd.Flags().StringArrayVarP(&stackPath, "file", "f", []string{}, "path to the stack manifest file")
	cmd.Flags().StringVarP(&name, "name", "", "", "overwrites the stack name")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is destroyed")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volumes")
	return cmd
}
