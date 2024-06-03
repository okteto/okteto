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
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// DeployCommand has all the namespaces subcommands
type DeployCommand struct {
	K8sClient        kubernetes.Interface
	analyticsTracker buildTrackerInterface
	insights         buildTrackerInterface
	Config           *rest.Config
	ioCtrl           *io.Controller
	DivertDriver     divert.Driver
	IsInsideDeploy   bool
}

// deploy deploys a stack
func deploy(ctx context.Context, at, insights buildTrackerInterface, ioCtrl *io.Controller) *cobra.Command {
	options := &stack.DeployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy [service...]",
		Short: "Deploy a compose",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto stack deploy' is deprecated in favor of 'okteto deploy', and will be removed in a future version")
			options.ServicesToDeploy = args

			options.StackPaths = loadComposePaths(options.StackPaths)
			if len(options.StackPaths) == 1 {
				workdir := filesystem.GetWorkdirFromManifestPath(options.StackPaths[0])
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				options.StackPaths[0] = filesystem.GetManifestPathFromWorkdir(options.StackPaths[0], workdir)
			}
			s, err := contextCMD.LoadStackWithContext(ctx, options.Name, options.Namespace, options.StackPaths, afero.NewOsFs())
			if err != nil {
				return err
			}
			c, config, err := okteto.NewK8sClientProvider().Provide(okteto.GetContext().Cfg)
			if err != nil {
				return err
			}
			dc := &DeployCommand{
				K8sClient:        c,
				Config:           config,
				analyticsTracker: at,
				insights:         insights,
				ioCtrl:           ioCtrl,
				DivertDriver:     divert.NewNoop(),
			}
			return dc.RunDeploy(ctx, s, options)
		},
	}

	tenMinutes := 10 * time.Minute

	cmd.Flags().StringArrayVarP(&options.StackPaths, "file", "f", []string{}, "path to the compose manifest files. If more than one is passed the latest will overwrite the fields from the previous")
	cmd.Flags().StringVarP(&options.Name, "name", "", "", "overwrites the compose name")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the compose namespace where the compose is deployed")
	cmd.Flags().BoolVarP(&options.ForceBuild, "build", "", false, "build images before starting any compose service")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "", false, "wait until a minimum number of containers are in a ready state for every service")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", tenMinutes, "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringVarP(&options.Progress, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output (default \"tty\")")
	return cmd
}

// RunDeploy runs the deploy command sequence
func (c *DeployCommand) RunDeploy(ctx context.Context, s *model.Stack, options *stack.DeployOptions) error {

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
			if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: s.Namespace}); err != nil {
				return err
			}
		}
	}

	analytics.TrackStackWarnings(s.Warnings.NotSupportedFields)

	if len(options.ServicesToDeploy) == 0 {
		definedServices := []string{}
		for serviceName := range s.Services {
			definedServices = append(definedServices, serviceName)
		}
		options.ServicesToDeploy = definedServices
	}

	stackDeployer := &stack.Stack{
		K8sClient:        c.K8sClient,
		Config:           c.Config,
		AnalyticsTracker: c.analyticsTracker,
		IoCtrl:           c.ioCtrl,
		Divert:           c.DivertDriver,
		Insights:         c.insights,
	}
	err := stackDeployer.Deploy(ctx, s, options)

	analytics.TrackDeployStack(err == nil, s.IsCompose, utils.IsOktetoRepo())
	if err != nil {
		return err
	}
	oktetoLog.Success("Compose '%s' successfully deployed", s.Name)

	if !(!env.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) || !c.IsInsideDeploy) {
		if err := stack.ListEndpoints(ctx, s); err != nil {
			return err
		}
	}
	return nil
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
