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
	"os"
	"path/filepath"
	"strings"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// destroyRunner interface with the operations needed to execute the destroy operations
type destroyRunner interface {
	RunDestroy(params deployable.DestroyParameters) error
}

// DestroyOptions flags accepted by the remote-run destroy command
type DestroyOptions struct {
	Name         string
	Variables    []string
	ForceDestroy bool
}

// DestroyCommand struct with the dependencies needed to run the destroy operation
type DestroyCommand struct {
	runner        destroyRunner
	oktetoContext *okteto.ContextStateless
}

// Destroy starts the destroy command remotely. This is the command executed in the
// remote environment when destroy deploy is executed with the remote flag
func Destroy(ctx context.Context) *cobra.Command {
	options := &DestroyOptions{}
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "This command is the one in charge of executing the custom commands for the destroy operation when okteto destroy is executed remotely",
		Long: `This command is the one in charge of executing the custom commands for the destroy operation when okteto destroy is executed remotely.

The deployable entity is received as a base64 encoded string in the OKTETO_DEPLOYABLE environment variable. The deployable entity is a yaml file that contains the following fields:

commands:
- name: Echo deploy variable
  command: echo "This is a deploy variable ${DEPLOY_VARIABLE}"

It is important that this command does the minimum and must not do calculations that the destroy triggering it already does.
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

			if !oktetoContext.IsOktetoCluster() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			// We need to store the kubeconfig of the current Okteto context locally, so commands
			// would use the expected
			kubeconfigPath := getTempKubeConfigFile("destroy", options.Name)
			if err := kubeconfig.Write(oktetoContext.GetCurrentCfg(), kubeconfigPath); err != nil {
				return err
			}
			os.Setenv("KUBECONFIG", kubeconfigPath)
			defer os.Remove(kubeconfigPath)

			dep, err := getDeployable()
			if err != nil {
				return fmt.Errorf("could not read information to be destroyed: %w", err)
			}

			// Set the default values for the external resources environment variables (endpoints)
			for name, external := range dep.External {
				external.SetDefaults(name)
			}

			runner := &deployable.DestroyRunner{
				Executor: executor.NewExecutor(oktetoLog.GetOutputFormat(), false, ""),
			}
			if err != nil {
				return fmt.Errorf("could not initialize the command properly: %w", err)
			}

			os.Setenv(constants.OktetoNameEnvVar, options.Name)

			params := deployable.DestroyParameters{
				Name:         options.Name,
				Namespace:    oktetoContext.GetNamespace(),
				Deployable:   dep,
				Variables:    options.Variables,
				ForceDestroy: options.ForceDestroy,
			}

			c := &DestroyCommand{
				runner:        runner,
				oktetoContext: oktetoContext,
			}

			return c.Run(params)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVar(&options.ForceDestroy, "force-destroy", false, "forces the development environment to be destroyed even if there is an error executing the custom destroy commands")
	return cmd
}

func (c *DestroyCommand) Run(params deployable.DestroyParameters) error {
	// Token should be always masked from the logs
	oktetoLog.AddMaskedWord(c.oktetoContext.GetCurrentToken())
	keyValueVarParts := 2
	// We mask all the variables received in the command
	for _, variable := range params.Variables {
		varParts := strings.SplitN(variable, "=", keyValueVarParts)
		if len(varParts) >= keyValueVarParts && strings.TrimSpace(varParts[1]) != "" {
			oktetoLog.AddMaskedWord(varParts[1])
		}
	}

	return c.runner.RunDestroy(params)
}

func getTempKubeConfigFile(operation string, name string) string {
	tempKubeconfigFileName := fmt.Sprintf("kubeconfig-%s-%s-%d", operation, name, time.Now().UnixMilli())
	return filepath.Join(config.GetOktetoHome(), tempKubeconfigFileName)
}
