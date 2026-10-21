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

package cmd

import (
	"github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// oktetoClientProvider provides an okteto client ready to use or fail
type oktetoClientProvider interface {
	Provide(...okteto.Option) (types.OktetoInterface, error)
}

// Kubeconfig fetch credentials for a cluster namespace
func Kubeconfig(okClientProvider oktetoClientProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Download credentials for the Kubernetes cluster selected via 'okteto context'",
		Long: `Download credentials for the Kubernetes cluster selected via 'okteto context'.

Generated kubeconfig file uses a credential plugin to get the cluster credentials via Okteto backend that requires the Okteto CLI to be in the PATH. Learn more about how to use the Kubernetes credentials at https://www.okteto.com/docs/core/credentials/kubernetes-credentials#using-your-kubernetes-credentials.
`,
		Args: utils.NoArgsAccepted("https://okteto.com/docs/reference/okteto-cli/#kubeconfig"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return context.UpdateKubeconfigCMD(okClientProvider).RunE(cmd, args)
		},
	}
	return cmd
}
