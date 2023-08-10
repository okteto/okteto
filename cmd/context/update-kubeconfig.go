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
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

			// Run context command to get the Cfg into Okteto Context
			if err := NewContextCommand().Run(ctx, &ContextOptions{}); err != nil {
				return err
			}

			okCtx := okteto.Context()
			kubeconfigPath := config.GetKubeconfigPath()
			kubetokenEnabled := false

			okClient, err := okteto.NewOktetoClient()
			if err != nil {
				return err
			}
			if err := okClient.Kubetoken().CheckService(okCtx.Name, okCtx.Namespace); err == nil {
				kubetokenEnabled = true
			}
			return ExecuteUpdateKubeconfig(okCtx, kubeconfigPath, kubetokenEnabled)
		},
	}

	return cmd
}

func ExecuteUpdateKubeconfig(okContext *okteto.OktetoContext, kubeconfigPaths []string, kubetokenEnabled bool) error {
	contextName := okContext.Name
	if okContext.IsOkteto {
		contextName = okteto.UrlToKubernetesContext(contextName)

		if okContext.IsStoredAsInsecure {
			certPEM, err := base64.StdEncoding.DecodeString(okteto.Context().Certificate)
			if err != nil {
				oktetoLog.Debugf("couldn't decode context certificate from base64: %s", err)
				return err
			}
			okContext.Cfg.Clusters[contextName].CertificateAuthorityData = certPEM
		}

		if kubetokenEnabled {
			updateUserAuthInfoWithExec(okContext, okContext.UserID)
		}
	}

	if err := kubeconfig.Write(okContext.Cfg, kubeconfigPaths[0]); err != nil {
		return err
	}
	oktetoLog.Success("Updated kubernetes context '%s/%s' in '%s'", contextName, okContext.Namespace, kubeconfigPaths)

	return nil
}

func updateUserAuthInfoWithExec(okCtx *okteto.OktetoContext, userID string) {
	if okCtx.Cfg.AuthInfos == nil {
		okCtx.Cfg.AuthInfos = clientcmdapi.NewConfig().AuthInfos
		okCtx.Cfg.AuthInfos[userID] = clientcmdapi.NewAuthInfo()
	}

	if token := okCtx.Cfg.AuthInfos[userID].Token; token != "" {
		okCtx.Cfg.AuthInfos[userID].Token = ""
	}

	okCtx.Cfg.AuthInfos[userID].Exec = getExecConfigForContextAndNamespace(okCtx.Name, okCtx.Namespace)
}

// getExecConfigForContextAndNamespace returns ExecConfig with kubetoken command including namespace and context flags
func getExecConfigForContextAndNamespace(context, namespace string) *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion:         "client.authentication.k8s.io/v1",
		Command:            "okteto",
		Args:               []string{"kubetoken", "--context", context, "--namespace", namespace},
		InstallHint:        "Okteto needs to be installed and in your PATH to use this context. Please visit https://www.okteto.com/docs/getting-started/ for more information.",
		InteractiveMode:    "Never",
		ProvideClusterInfo: true,
	}
}
