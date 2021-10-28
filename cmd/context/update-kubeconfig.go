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

package context

import (
	"context"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// UpdateKubeconfig all contexts managed by okteto
func UpdateKubeconfigCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-kubeconfig",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#context"),
		Short: "Downloads k8s credentials for the current context",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := Run(ctx, &ContextOptions{}); err != nil {
				return err
			}

			if err := executeUpdateKubeconfig(ctx); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func executeUpdateKubeconfig(ctx context.Context) error {
	if err := kubeconfig.Write(okteto.Context().Cfg, config.GetKubeconfigPath()); err != nil {
		return err
	}
	k8sContext := okteto.Context().Name
	if okteto.Context().IsOkteto {
		k8sContext = okteto.UrlToKubernetesContext(k8sContext)
	}
	log.Information("Current kubernetes context '%s/%s' in '%s'", k8sContext, okteto.Context().Namespace, config.GetKubeconfigPath())
	return nil
}
