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

package kubetoken

import (
	"context"
	"fmt"
	"os"
	"path"

	contextCMD "github.com/okteto/okteto/cmd/context"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/okteto/kubetoken"
	"github.com/spf13/cobra"
)

const oktetoTokenCacheFileName = "okteto_auth_plugin_cache.json"

func KubeToken() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubetoken <context> <namespace>",
		Short: "Print Kubernetes cluster credentials in ExecCredential format.",
		Long: `Print Kubernetes cluster credentials in ExecCredential format.
You can find more information on 'ExecCredential' and 'client side authentication' at (https://kubernetes.io/docs/reference/config-api/client-authentication.v1/) and  https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins`,
		Hidden: true,
		Args:   cobra.ExactArgs(2),
	}

	cmd.RunE = func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		context := args[0]
		namespace := args[1]

		cacheFileName := path.Join(config.GetDotKubeFolder(), oktetoTokenCacheFileName)
		cache := kubetoken.NewCache(cacheFileName)
		// Return early if we have a valid token in the cache before we run the context command to improve performance
		if token, err := cache.Get(context, namespace); err == nil && token != "" {
			cmd.Print(token)
			return nil
		} else {
			log.Debugf("failed to get token from cache: %w", err)
		}

		err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{
			Context:   context,
			Namespace: namespace,
		})
		if err != nil {
			return err
		}

		if !okteto.Context().IsOkteto {
			return errors.ErrContextIsNotOktetoCluster
		}

		c, err := kubetoken.NewClient(okteto.Context().Name, okteto.Context().Token, namespace, cache)
		if err != nil {
			return fmt.Errorf("failed to initialize the kubetoken client: %w", err)
		}

		out, err := c.GetKubeToken()
		if err != nil {
			return fmt.Errorf("failed to get the kubetoken: %w", err)
		}

		cmd.Print(out)
		return nil
	}

	cmd.SetOut(os.Stdout)

	return cmd
}
