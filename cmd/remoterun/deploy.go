// Copyright 2024 The Okteto Authors
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

package remoterun

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	deployCMD "github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/pkg/deployable"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// deployRunner interface with the operations needed to execute the deploy operations
type deployRunner interface {
	RunDeploy(ctx context.Context, params deployable.DeployParameters) error
}

// DeployOptions flags accepted by the remote-run deploy command
type DeployOptions struct {
	Name      string
	Variables []string
}

// DeployCommand struct with the dependencies needed to run the deploy operation
type DeployCommand struct {
	runner deployRunner
}

// Deploy starts the deploy command remotely. This is the command executed in the
// remote environment when okteto deploy is executed with the remote flag
func Deploy(ctx context.Context, k8sLogger *io.K8sLogger) *cobra.Command {
	options := &DeployOptions{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "This command is the one in charge of deploying a deployable entity in the remote environment when okteto deploy is executed with remote flag",
		Long: `This command is the one in charge of deploying a deployable entity in the remote environment when okteto deploy is executed with remote flag.

The deployable entity is received as a base64 encoded string in the OKTETO_DEPLOYABLE environment variable. The deployable entity is a yaml file that contains the following fields:

commands:
- name: Echo deploy variable
  command: echo "This is a deploy variable ${DEPLOY_VARIABLE}"
...
external:
  fake:
    icon: dashboard
    notes: docs/fake.md
    endpoints:
    - name: api
      url: https://fake-service
...
divert:
  driver: istio
  namespace: ns-1
  service: service-b
  virtualService: service-b
  hosts:
    - virtualService: service-a
	  namespace: ns-a
    - virtualService: service-b
	  namespace: ${OKTETO_NAMESPACE}
...

It is important that this command does the minimum and must not do calculations that the deploy triggering it already does. For example, this command must not build any image to be used during the deployment
`,
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.Name == "" {
				return fmt.Errorf("--name is required")
			}

			oktetoContext, err := contextCMD.NewContextCommand().RunStateless(ctx, &contextCMD.Options{})
			if err != nil {
				return err
			}

			dep, err := getDeployable()
			if err != nil {
				return fmt.Errorf("could not read information to be deployed: %w", err)
			}

			// Set the default values for the external resources environment variables (endpoints)
			for name, external := range dep.External {
				external.SetDefaults(name)
			}

			k8sClientProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)
			cmapHandler := deployCMD.NewConfigmapHandler(k8sClientProvider, k8sLogger)

			runner, err := deployable.NewDeployRunnerForRemote(
				options.Name,
				false,
				cmapHandler,
				k8sClientProvider,
				model.GetAvailablePort,
				k8sLogger,
			)
			if err != nil {
				return fmt.Errorf("could not initialize the command properly: %w", err)
			}

			params := deployable.DeployParameters{
				Name:      options.Name,
				Namespace: oktetoContext.GetCurrentNamespace(),
				// For the remote command, the manifest path is the current directory
				ManifestPath: ".",
				Deployable:   dep,
				Variables:    options.Variables,
			}

			c := &DeployCommand{
				runner: runner,
			}

			return c.Run(ctx, params)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	return cmd
}

func (c *DeployCommand) Run(ctx context.Context, params deployable.DeployParameters) error {
	err := c.runner.RunDeploy(ctx, params)
	oktetoLog.DisableMasking()
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return err
}
