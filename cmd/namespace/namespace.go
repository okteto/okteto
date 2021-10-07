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

package namespace

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Namespace fetch credentials for a cluster namespace
func Namespace(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace [name]",
		Short: "Downloads k8s credentials for a namespace",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#namespace"),
		RunE: func(cmd *cobra.Command, args []string) error {

			namespace := ""
			if len(args) > 0 {
				namespace = args[0]
			}

			if err := contextCMD.Init(ctx); err != nil {
				return err
			}

			if !okteto.IsOktetoContext() {
				return errors.ErrContextIsNotOktetoCluster
			}

			err := RunNamespace(ctx, namespace)
			analytics.TrackNamespace(err == nil)
			return err
		},
	}
	return cmd
}

// RunNamespace starts the kubeconfig sequence
func RunNamespace(ctx context.Context, namespace string) error {

	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	cred, err := oktetoClient.GetCredentials(ctx)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = cred.Namespace
	}

	hasAccess, err := hasAccessToNamespace(ctx, namespace)
	if err != nil {
		return err
	}
	if !hasAccess {
		return fmt.Errorf(errors.ErrNamespaceNotFound, namespace)
	}

	octx := okteto.Context()
	kubeconfigFile := config.GetKubeconfigPath()
	if err := okteto.SetKubeContext(cred, kubeconfigFile, namespace, octx.UserID, okteto.UrlToContext(octx.Name)); err != nil {
		return err
	}

	cfg := client.GetKubeconfig(kubeconfigFile)
	u := octx.ToUser()

	if err := okteto.SaveOktetoClusterContext(octx.Name, u, namespace, cfg); err != nil {
		return err
	}

	log.Success("Updated context '%s': current namespace '%s'", octx.Name, namespace)
	return nil
}

func hasAccessToNamespace(ctx context.Context, namespace string) (bool, error) {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return false, err
	}
	spaces, err := oktetoClient.ListNamespaces(ctx)
	if err != nil {
		return false, err
	}

	for i := range spaces {
		if spaces[i].ID == namespace {
			return true, nil
		}
	}
	return false, nil
}
