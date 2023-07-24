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

package context

import (
	"context"
	"encoding/base64"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// UpdateKubeconfigCMD all contexts managed by okteto
func UpdateKubeconfigCMD() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "update-kubeconfig",
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#kubeconfig"),
		Short:  "Download credentials for the Kubernetes cluster selected via 'okteto context'",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := NewContextCommand().Run(ctx, &ContextOptions{}); err != nil {
				return err
			}

			if err := ExecuteUpdateKubeconfig(); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func ExecuteUpdateKubeconfig() error {
	ctx := okteto.Context()
	k8sContext := okteto.Context().Name

	if ctx.IsOkteto {
		k8sContext = okteto.UrlToKubernetesContext(k8sContext)
		if ctx.IsStoredAsInsecure {
			certPEM, err := base64.StdEncoding.DecodeString(okteto.Context().Certificate)
			if err != nil {
				oktetoLog.Debugf("couldn't decode context certificate from base64: %s", err)
				return err
			}
			ctx.Cfg.Clusters[k8sContext].CertificateAuthorityData = certPEM
		}
	}
	if err := kubeconfig.Write(ctx.Cfg, config.GetKubeconfigPath()[0]); err != nil {
		return err
	}

	oktetoLog.Success("Updated kubernetes context '%s/%s' in '%s'", k8sContext, ctx.Namespace, config.GetKubeconfigPath())
	return nil
}
