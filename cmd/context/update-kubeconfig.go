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
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// kubeconfigController has all the functions that the context command needs to update the kubeconfig stored in the okteto context
// and the ones to store in the kubeconfig file
type kubeconfigController interface {
	kubeconfigTokenController

	updateOktetoContextExec(*okteto.Context) error
}

type KubeconfigCMD struct {
	kubetokenController kubeconfigController
}

// newKubeconfigController creates a new command to update the kubeconfig stored in the okteto context
func newKubeconfigController(okClientProvider oktetoClientProvider) *KubeconfigCMD {
	var kubetokenController kubeconfigController
	if env.LoadBoolean(OktetoUseStaticKubetokenEnvVar) {
		kubetokenController = newStaticKubetokenController()
	} else {
		kubetokenController = newDynamicKubetokenController(okClientProvider)
	}
	return &KubeconfigCMD{
		kubetokenController: kubetokenController,
	}
}

// UpdateKubeconfigCMD all contexts managed by okteto
func UpdateKubeconfigCMD(okClientProvider oktetoClientProvider) *cobra.Command {
	kc := newKubeconfigController(okClientProvider)
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "update-kubeconfig",
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#kubeconfig"),
		Short:  "Download credentials for the Kubernetes cluster selected via 'okteto context'",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Run context command to get the Cfg into Okteto GetContext
			if err := NewContextCommand(withKubeTokenController(kc.kubetokenController)).Run(ctx, &Options{}); err != nil {
				return err
			}

			return kc.execute(okteto.GetContext(), config.GetKubeconfigPath())
		},
	}

	return cmd
}

func (k *KubeconfigCMD) execute(okCtx *okteto.Context, kubeconfigPaths []string) error {
	contextName := okCtx.Name
	if okCtx.IsOkteto {
		contextName = okteto.UrlToKubernetesContext(contextName)
		if err := updateCfgClusterCertificate(contextName, okCtx); err != nil {
			return err
		}

		err := k.kubetokenController.updateOktetoContextExec(okCtx)
		if err != nil {
			oktetoLog.Infof("failed to update okteto kubeconfig: %s", err)
		}
	}

	if err := kubeconfig.Write(okCtx.Cfg, kubeconfigPaths[0]); err != nil {
		return err
	}

	oktetoLog.Success("Updated kubernetes context '%s/%s' in '%s'", contextName, okCtx.Namespace, kubeconfigPaths)
	return nil
}

func updateCfgClusterCertificate(contextName string, okContext *okteto.Context) error {
	if !okContext.IsStoredAsInsecure {
		return nil
	}

	subdomain := strings.TrimPrefix(okContext.Registry, "registry.")
	if okContext.Cfg.Clusters[contextName].Server != fmt.Sprintf("kubernetes.%s", subdomain) {
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
