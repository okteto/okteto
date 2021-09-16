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
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

// Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	options := stack.StackDeployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy [service...]",
		Short: "Deploys a stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			ctx := context.Background()
			if err := utils.LoadEnvironment(ctx, true); err != nil {
				return err
			}

			s, err := utils.LoadStack(options.Name, options.StackPath)
			if err != nil {
				return err
			}
			analytics.TrackStackWarnings(s.Warnings.NotSupportedFields)

			if len(args) > 0 {
				options.ServicesToDeploy = args
			} else {
				definedSvcs := make([]string, 0)
				for svcName := range s.Services {
					definedSvcs = append(definedSvcs, svcName)
				}
				options.ServicesToDeploy = definedSvcs
			}

			if err := s.UpdateNamespace(options.Namespace); err != nil {
				return err
			}

			err = stack.Deploy(ctx, s, options)
			analytics.TrackDeployStack(err == nil, s.IsCompose)
			if err == nil {
				log.Success("Stack '%s' successfully deployed", s.Name)
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&options.StackPath, "file", "f", utils.DefaultStackManifest, "path to the stack manifest file")
	cmd.Flags().StringVarP(&options.Name, "name", "", "", "overwrites the stack name")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is deployed")
	cmd.Flags().BoolVarP(&options.ForceBuild, "build", "", false, "build images before starting any Stack service")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "", false, "wait until a minimum number of containers are in a ready state for every service")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", (10 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	return cmd
}
