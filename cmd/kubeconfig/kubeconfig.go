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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/spf13/cobra"
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

			err := RunKubeconfig(ctx)
			analytics.TrackKubeconfig(err == nil)
			return err
		},
	}
	return cmd
}

// RunKubeconfig starts the kubeconfig sequence
func RunKubeconfig(_ context.Context) error {

	// oktetoKubeConfig, err := getClientConfig.GetKubeconfig(config.GetKubeconfigFile())
	// if err != nil {
	// 	return err
	// }

	// ctxToCopy := oktetoKubeConfig.CurrentContext

	// userKubeConfigFile := config.GetKubeConfigFile()
	// err = okteto.SetContextFromConfigFields(userKubeConfigFile, ctxToCopy, oktetoKubeConfig.AuthInfos[ctxToCopy], oktetoKubeConfig.Clusters[ctxToCopy], oktetoKubeConfig.Contexts[ctxToCopy], oktetoKubeConfig.Extensions[ctxToCopy])
	// if err != nil {
	// 	return err
	// }
	// log.Success("Updated context '%s' in '%s'", ctxToCopy, userKubeConfigFile)
	return nil
}
