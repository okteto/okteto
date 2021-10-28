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
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Create adds a new cluster to okteto context
func CreateCMD() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Use:   "create [cluster-url]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#context"),
		Short: "Add a context",
		Long: `Add a context

A context is a group of cluster access parameters. Each context contains a Kubernetes cluster, a user, and a namespace.
The current context is the default cluster/namespace for any Okteto CLI command.

You need to specify the URL of your Okteto Enterprise. For example, run:

	$ okteto context create https://cloud.okteto.com

to configure your context to access Okteto Cloud.

Your browser will ask for your authentication to retrieve your API token.

If you need to automate authentication or if you don't want to use browser-based authentication, use the "--token" parameter:

	$ okteto context create https://cloud.okteto.com --token ${OKTETO_TOKEN}
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			ctxOptions.Context = args[0]
			ctxOptions.Context = okteto.AddSchema(ctxOptions.Context)
			ctxOptions.Context = strings.TrimSuffix(ctxOptions.Context, "/")
			ctxOptions.isOkteto = true

			err := UseContext(ctx, ctxOptions)
			analytics.TrackContext(err == nil)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "namespace of your okteto context")
	cmd.Flags().StringVarP(&ctxOptions.Builder, "builder", "b", "", "url of the builder service")
	return cmd
}

func UseContext(ctx context.Context, ctxOptions *ContextOptions) error {
	created := false

	ctxStore := okteto.ContextStore()
	if okCtx, ok := ctxStore.Contexts[ctxOptions.Context]; ok && okCtx.IsOkteto {
		ctxOptions.isOkteto = true
	}

	if okCtx, ok := ctxStore.Contexts[okteto.AddSchema(ctxOptions.Context)]; ok && okCtx.IsOkteto {
		ctxOptions.Context = okteto.AddSchema(ctxOptions.Context)
		ctxOptions.isOkteto = true
	}

	if !ctxOptions.isOkteto {
		if !isValidCluster(ctxOptions.Context) {
			log.Fail("%s: invalid okteto context", ctxOptions.Context)
			ctxOptions = &ContextOptions{}
			ctxOptions.isCtxCommand = true
			err := Run(ctx, ctxOptions)
			analytics.TrackContext(err == nil)
			return err
		}

		transformedCtx := okteto.K8sContextToOktetoUrl(ctx, ctxOptions.Context, ctxOptions.Namespace)
		if transformedCtx != ctxOptions.Context {
			ctxOptions.Context = transformedCtx
			ctxOptions.isOkteto = true
		}
	}

	if okCtx, ok := ctxStore.Contexts[ctxOptions.Context]; !ok {
		ctxStore.Contexts[ctxOptions.Context] = &okteto.OktetoContext{Name: ctxOptions.Context}
		created = true
	} else if ctxOptions.Token == "" {
		//this is to avoid login with the browser again if we already have a valid token
		ctxOptions.Token = okCtx.Token
	}

	ctxStore.CurrentContext = ctxOptions.Context

	if ctxOptions.isOkteto {
		if err := initOktetoContext(ctx, ctxOptions); err != nil {
			return err
		}
	} else {
		if err := initKubernetesContext(ctxOptions); err != nil {
			return err
		}
	}
	if err := okteto.WriteOktetoContextConfig(); err != nil {
		return err
	}
	if created && ctxOptions.isOkteto {
		log.Success("Context '%s' created", okteto.RemoveSchema(ctxOptions.Context))
	}

	if ctxOptions.isCtxCommand {
		log.Success("Using context %s @ %s", okteto.Context().Namespace, okteto.RemoveSchema(ctxStore.CurrentContext))
	}

	return nil
}
