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
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func KubeToken() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "kubetoken",
		Short:  "Gets a token to access the Kubernetes API with client authentication",
		Hidden: true,
	}

	cmd.RunE = func(*cobra.Command, []string) error {
		ctx := context.Background()

		cfg := kubeconfig.Get(config.GetKubeconfigPath())
		if cfg == nil {
			return fmt.Errorf("failed to get the kubeconfig file. Please, check that your kubeconfig file is valid and in path")
		}

		err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{
			Context: cfg.CurrentContext,
		})
		if err != nil {
			return err
		}

		if !okteto.Context().IsOkteto {
			return errors.ErrContextIsNotOktetoCluster
		}

		c, err := okteto.NewKubeTokenClient()
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

	cmd.SetOut(os.Stdout)

	return cmd
}
