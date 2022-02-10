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

package stack

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/stack"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	options := &stack.StackDeployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy [service...]",
		Short: "Deploy a stack",
		RunE: func(cmd *cobra.Command, args []string) error {

			options.StackPath = loadComposePaths(options.StackPath)
			s, err := contextCMD.LoadStackWithContext(ctx, options.Name, options.Namespace, options.StackPath)
			if err != nil {
				return err
			}

			if okteto.IsOkteto() {
				create, err := utils.ShouldCreateNamespace(ctx, s.Namespace)
				if err != nil {
					return err
				}
				if create {
					nsCmd, err := namespace.NewCommand()
					if err != nil {
						return err
					}
					nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: s.Namespace})
				}
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

			err = stack.Deploy(ctx, s, options)

			analytics.TrackDeployStack(err == nil, s.IsCompose, utils.IsOktetoRepo())
			if err == nil {
				oktetoLog.Success("Stack '%s' successfully deployed", s.Name)
			}

			if !utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
				if err := stack.ListEndpoints(ctx, s, ""); err != nil {
					return err
				}
			}

			return err
		},
	}
	cmd.Flags().StringArrayVarP(&options.StackPath, "file", "f", []string{}, "path to the stack manifest files. If more than one is passed the latest will overwrite the fields from the previous")
	cmd.Flags().StringVarP(&options.Name, "name", "", "", "overwrites the stack name")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is deployed")
	cmd.Flags().BoolVarP(&options.ForceBuild, "build", "", false, "build images before starting any Stack service")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "", false, "wait until a minimum number of containers are in a ready state for every service")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", (10 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringVarP(&options.Progress, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output (default \"tty\")")
	return cmd
}

func splitComposeFileEnv(value string) []string {
	if runtime.GOOS == "windows" {
		return strings.Split(value, ";")
	}
	return strings.Split(value, ":")
}

func loadComposePaths(paths []string) []string {
	composeEnv, present := os.LookupEnv(model.ComposeFileEnvVar)
	if len(paths) == 0 && present {
		paths = splitComposeFileEnv(composeEnv)
	}
	return paths
}
