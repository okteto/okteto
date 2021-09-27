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
	"github.com/okteto/okteto/cmd/utils"
	"github.com/spf13/cobra"
)

// SetNamespace sets the namespace in current okteto context.
func SetNamespace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-namespace",
		Args:  utils.ExactArgsAccepted(1, "https://okteto.com/docs/reference/cli/#context"),
		Short: "Set the namespace of current okteto context",
		RunE: func(cmd *cobra.Command, args []string) error {
			// ctx := context.Background()
			// namespace := args[0]
			// oktetoContext := okteto.GetCurrentContext()
			// if oktetoContext.IsOktetoCluster() {
			// 	hasAccess, err := utils.HasAccessToNamespace(ctx, namespace)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	if !hasAccess {
			// 		return fmt.Errorf(errors.ErrNamespaceNotFound, namespace)
			// 	}
			// }
			// kubeconfigFile := config.GetKubeconfigFile()
			// cfg, err := okteto.GetKubeconfig(kubeconfigFile)
			// if err != nil {
			// 	return err
			// }

			// cfg.CurrentNamespace = namespace
			// //TODO: save kubeconfig

			// oktetoContext.Namespace = namespace
			// //TODO: save okteto context

			// log.Success("Context '%s' namespace has been updated to '%s'", oktetoContext.Name, namespace)
			return nil
		},
	}

	return cmd
}
