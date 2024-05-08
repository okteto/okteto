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
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// TestOptions flags accepted by the remote-run test command
type TestOptions struct {
	Name      string
	Variables []string
}

// Test starts the test command remotely. This is the command executed in the
// remote environment when running okteto test
func Test(ctx context.Context) *cobra.Command {
	options := &TestOptions{}
	cmd := &cobra.Command{
		Use:   "test",
		Short: "This command is the one in charge of executing the custom commands for the test operation for okteto test",
		Long: `This command is the one in charge of executing the custom commands for the test operation for okteto test.

The deployable entity is received as a base64 encoded string in the OKTETO_DEPLOYABLE environment variable. The deployable entity is a yaml file that contains the following fields:

commands:
- name: Echo deploy variable
  command: echo "This is a deploy variable ${DEPLOY_VARIABLE}"
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

			// We need to store the kubeconfig of the current Okteto context locally, so commands
			// would use the expected
			kubeconfigPath := getTempKubeConfigFile("test", options.Name)
			if err := kubeconfig.Write(oktetoContext.GetCurrentCfg(), kubeconfigPath); err != nil {
				return err
			}
			os.Setenv("KUBECONFIG", kubeconfigPath)
			defer os.Remove(kubeconfigPath)

			dep, err := getDeployable()
			if err != nil {
				return fmt.Errorf("could not read information for tests: %w", err)
			}

			// Set the default values for the external resources environment variables (endpoints)
			for name, external := range dep.External {
				external.SetDefaults(name)
			}

			runner := &deployable.TestRunner{
				Executor: executor.NewExecutor(oktetoLog.GetOutputFormat(), false, ""),
				Fs:       afero.NewOsFs(),
			}

			os.Setenv(constants.OktetoNameEnvVar, options.Name)

			params := deployable.TestParameters{
				Name:       options.Name,
				Namespace:  oktetoContext.GetCurrentNamespace(),
				Deployable: dep,
				Variables:  options.Variables,
			}

			// Token should be always masked from the logs
			oktetoLog.AddMaskedWord(okteto.GetContext().Token)
			keyValueVarParts := 2
			// We mask all the variables received in the command
			for _, variable := range params.Variables {
				varParts := strings.SplitN(variable, "=", keyValueVarParts)
				if len(varParts) >= keyValueVarParts && strings.TrimSpace(varParts[1]) != "" {
					oktetoLog.AddMaskedWord(varParts[1])
				}
			}

			return runner.RunTest(params)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "test run name")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	return cmd
}
