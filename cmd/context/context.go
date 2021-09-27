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
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/cmd/kubeconfig"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	contextCmd "github.com/okteto/okteto/pkg/cmd/context"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

type ContextOptions struct {
	Token     string
	Namespace string
}

// Context points okteto to a cluster.
func Context() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Use:     "context [url|k8s-context]",
		Aliases: []string{"ctx"},
		Args:    utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#context"),
		Short:   "Manage your okteto context",
		Long: `Manage your okteto context

	A context is a group of access parameters. Each context contains a Kubernetes cluster, a user, and a namespace.
	The current context is the cluster that is currently the default for the Okteto CLI. All "okteto" commands run against that cluster.

	If you want to log into an Okteto Enterprise instance, specify a URL. For example, run

		$ okteto context https://cloud.okteto.com

	to configure your context to acces Okteto Cloud.

	Your browser will ask for your authentication to retrieve your API token.

	If you need to automate authentication or if you don't want to use browser-based authentication, use the "--token" parameter:

		$ okteto context https://cloud.okteto.com --token ${OKTETO_TOKEN}

	You can also specify the name of a Kubernetes context with:

	    $ okteto context kubernetes_context_name

	Or show a list of available options with:

		$ okteto context
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			//TODO: fix this
			// if ctxOptions.Token == "" && k8Client.InCluster() {
			// 	return errors.ErrTokenFlagNeeded
			// }

			apiToken := os.Getenv("OKTETO_TOKEN")
			if ctxOptions.Token == "" {
				ctxOptions.Token = apiToken
			}

			var err error
			oktetoContext := os.Getenv("OKTETO_URL")
			if len(args) == 0 {
				if oktetoContext != "" {
					log.Infof("authenticating with OKTETO_URL")
				} else {
					log.Infof("authenticating with interactive context")
					oktetoContext, err = getContext(ctx, ctxOptions)
					if err != nil {
						return err
					}
				}
			} else {
				log.Infof("authenticating with context argument")
				oktetoContext = args[0]
			}

			err = runContext(ctx, oktetoContext, ctxOptions)
			analytics.TrackContext(err == nil)
			if err != nil {
				return err
			}

			kubeconfig.RunKubeconfig(ctx)
			return nil
		},
	}

	cmd.AddCommand(SetNamespace())
	cmd.AddCommand(List())
	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication.")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "namespace of your okteto context")
	return cmd
}

func runContext(ctx context.Context, oktetoContext string, ctxOptions *ContextOptions) error {

	if okteto.IsOktetoCluster(oktetoContext) {
		err := authenticateToOktetoCluster(ctx, oktetoContext, ctxOptions.Token)
		if err != nil {
			return err
		}
	} else {
		if !isValidCluster(oktetoContext) {
			return errors.UserError{
				E:    fmt.Errorf(errors.ErrInvalidContext, oktetoContext),
				Hint: fmt.Sprintf("Valid Kubernetes contexts are:\n      %s", strings.Join(getKubernetesContextList(), "\n      ")),
			}
		}
		err := contextCmd.CopyK8sClusterConfigToOktetoContext(oktetoContext)
		if err != nil {
			return err
		}
	}

	//TODO: save namespace flag
	return saveOktetoContext(ctx)

}

func getContext(ctx context.Context, ctxOptions *ContextOptions) (string, error) {
	clusters := []string{"Okteto Cloud", "Okteto Enterprise"}
	k8sClusters := getKubernetesContextList()
	clusters = append(clusters, k8sClusters...)
	oktetoContext, err := utils.AskForOptions(clusters, "Select the context you want to activate:")

	if err != nil {
		return "", err
	}

	if isOktetoCluster(oktetoContext) {
		oktetoContext = getOktetoClusterUrl(ctx, oktetoContext)
	}

	return oktetoContext, nil
}
