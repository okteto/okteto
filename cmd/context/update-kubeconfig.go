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
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// oktetoUseStaticKubetokenEnvVar is used to opt in to use static kubetoken
	oktetoUseStaticKubetokenEnvVar = "OKTETO_USE_STATIC_KUBETOKEN"
)

var (
	usingStaticKubetokenWarningMessage = fmt.Sprintf("Using static Kubernetes token due to env var: '%s'. This feature will be removed in the future. We recommend using a dynamic kubernetes token.", oktetoUseStaticKubetokenEnvVar)
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

			return ExecuteUpdateKubeconfig(okteto.Context(), config.GetKubeconfigPath(), okteto.NewOktetoClientProvider())
		},
	}

	return cmd
}

func ExecuteUpdateKubeconfig(okCtx *okteto.OktetoContext, kubeconfigPaths []string, okClientProvider types.OktetoClientProvider) error {
	contextName := okCtx.Name
	if okCtx.IsOkteto {
		contextName = okteto.UrlToKubernetesContext(contextName)
		if err := updateCfgClusterCertificate(contextName, okCtx); err != nil {
			return err
		}

		okClient, err := okClientProvider.Provide()
		if err != nil {
			return err
		}

		if utils.LoadBoolean(oktetoUseStaticKubetokenEnvVar) {
			removeExecFromCfg(okCtx)
			oktetoLog.Warning(usingStaticKubetokenWarningMessage)
		} else {
			if err := okClient.Kubetoken().CheckService(okCtx.Name, okCtx.Namespace); err == nil {
				updateCfgAuthInfoWithExec(okCtx)
			} else {
				oktetoLog.Debug("Error checking kubetoken service: %w", err)
			}
		}
	}

	if err := kubeconfig.Write(okCtx.Cfg, kubeconfigPaths[0]); err != nil {
		return err
	}

	oktetoLog.Success("Updated kubernetes context '%s/%s' in '%s'", contextName, okCtx.Namespace, kubeconfigPaths)
	return nil
}

func updateCfgClusterCertificate(contextName string, okContext *okteto.OktetoContext) error {
	if !okContext.IsStoredAsInsecure {
		return nil
	}

	certPEM, err := base64.StdEncoding.DecodeString(okContext.Certificate)
	if err != nil {
		oktetoLog.Debugf("couldn't decode context certificate from base64: %s", err)
		return err
	}
	okContext.Cfg.Clusters[contextName].CertificateAuthorityData = certPEM
	return nil
}

func updateCfgAuthInfoWithExec(okCtx *okteto.OktetoContext) {
	if okCtx.Cfg.AuthInfos == nil {
		okCtx.Cfg.AuthInfos = clientcmdapi.NewConfig().AuthInfos
		okCtx.Cfg.AuthInfos[okCtx.UserID] = clientcmdapi.NewAuthInfo()
	}

	if token := okCtx.Cfg.AuthInfos[okCtx.UserID].Token; token != "" {
		okCtx.Cfg.AuthInfos[okCtx.UserID].Token = ""
	}

	okCtx.Cfg.AuthInfos[okCtx.UserID].Exec = &clientcmdapi.ExecConfig{
		APIVersion:         "client.authentication.k8s.io/v1",
		Command:            "okteto",
		Args:               []string{"kubetoken", "--context", okCtx.Name, "--namespace", okCtx.Namespace},
		InstallHint:        "Okteto needs to be installed and in your PATH to use this context. Please visit https://www.okteto.com/docs/getting-started/ for more information.",
		InteractiveMode:    "Never",
		ProvideClusterInfo: true,
	}
}

func removeExecFromCfg(okCtx *okteto.OktetoContext) {
	if okCtx == nil || okCtx.UserID == "" || okCtx.Cfg == nil || okCtx.Cfg.AuthInfos == nil || okCtx.Cfg.AuthInfos[okCtx.UserID] == nil {
		return
	}

	okCtx.Cfg.AuthInfos[okCtx.UserID].Exec = nil
}
