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

package kubetoken

import (
	"context"
	"fmt"
	"os"

	contextCMD "github.com/okteto/okteto/cmd/context"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func KubeToken() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubetoken <context> <namespace>",
		Short: "Print Kubernetes cluster credentials in ExecCredential format.",
		Long: `Print Kubernetes cluster credentials in ExecCredential format.
You can find more information on 'ExecCredential' and 'client side authentication' at (https://kubernetes.io/docs/reference/config-api/client-authentication.v1/) and  https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins`,
		Hidden: true,
		Args:   cobra.NoArgs,
	}

	var namespace string
	var contextName string
	cmd.RunE = func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{
			Context:   contextName,
			Namespace: namespace,
		})
		if err != nil {
			return err
		}

		octx := okteto.Context()
		if !octx.IsOkteto {
			return errors.ErrContextIsNotOktetoCluster
		}

		c, err := okteto.NewKubeTokenClient(octx.Name, octx.Token, octx.Namespace)
		if err != nil {
			return fmt.Errorf("failed to initialize the kubetoken client: %w", err)
		}

		out, err := c.GetKubeToken()
		if err != nil {
			return fmt.Errorf("failed to get the kubetoken: %w", err)
		}

		cmd.Print(out)
		return nil
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "okteto context's namespace")
	cmd.Flags().StringVarP(&contextName, "context", "c", "", "okteto context's name")

	cmd.SetOut(os.Stdout)

	return cmd
}
