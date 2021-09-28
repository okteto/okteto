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

package kubeconfig

import (
	"context"
	"encoding/json"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Kubeconfig fetch credentials for a cluster namespace
func Kubeconfig(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Update your kubeconfig file with the credentials of the current okteto context",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#kubeconfig"),
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			err := RunKubeconfig()
			analytics.TrackKubeconfig(err == nil)
			return err
		},
	}
	return cmd
}

// RunKubeconfig starts the kubeconfig sequence
func RunKubeconfig() error {
	occ, err := okteto.GetContexts()
	if err != nil {
		return err
	}

	//TODO: index error (everywhere)
	kubeconfigBase64 := occ.Contexts[occ.CurrentContext].Kubeconfig
	var cfg clientcmdapi.Config
	if err := json.Unmarshal([]byte(kubeconfigBase64), &cfg); err != nil {
		return err
	}
	kubeconfigFile := config.GetKubeconfigPath()
	k8sCfg := client.GetKubeconfig(kubeconfigFile)
	currentContext := cfg.Contexts[cfg.CurrentContext]
	k8sCfg.CurrentContext = cfg.CurrentContext
	k8sCfg.AuthInfos[currentContext.AuthInfo] = cfg.AuthInfos[currentContext.AuthInfo]
	k8sCfg.Contexts[cfg.CurrentContext] = cfg.Contexts[cfg.CurrentContext]
	k8sCfg.Clusters[currentContext.Cluster] = cfg.Clusters[currentContext.Cluster]

	return client.SetKubeconfig(&cfg, kubeconfigFile)
}
