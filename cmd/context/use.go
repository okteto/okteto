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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// Use context points okteto to a cluster.
func Use() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Use:   "use [<url>|Kubernetes context]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#use"),
		Short: "Set the default context",
		Long: `Set the default context

A context is a group of cluster access parameters. Each context contains a Kubernetes cluster, a user, and a namespace.
The current context is the default cluster/namespace for any Okteto CLI command.

To set your default context, run the ` + "`okteto context use`" + ` command:

    $ okteto context use

This will prompt you to select one of your existing contexts or to create a new one.

You can also specify an Okteto URL:

    $ okteto context use https://cloud.okteto.com

Or a Kubernetes context:

    $ okteto context use kubernetes_context_name
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(args) == 1 {
				ctxOptions.Context = strings.TrimSuffix(args[0], "/")
			}

			ctxOptions.IsCtxCommand = true
			ctxOptions.Save = true
			err := Run(ctx, ctxOptions)
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
	cmd.Flags().BoolVarP(&ctxOptions.OnlyOkteto, "okteto", "", false, "only shows okteto cluster options")
	cmd.Flags().MarkHidden("okteto")
	return cmd
}

func Run(ctx context.Context, ctxOptions *ContextOptions) error {
	ctxStore := okteto.ContextStore()
	ctxOptions.initFromContext()
	ctxOptions.initFromEnvVars()

	if ctxOptions.Token == "" && kubeconfig.InCluster() && !isValidCluster(ctxOptions.Context) {
		return errors.ErrTokenFlagNeeded
	}

	if ctxOptions.Context == "" {
		if !ctxOptions.IsCtxCommand {
			log.Information("Okteto context is not initialized")
		}
		log.Infof("authenticating with interactive context")
		oktetoContext, err := getContext(ctx, ctxOptions)
		if err != nil {
			return err
		}
		ctxOptions.Context = oktetoContext
		ctxStore.CurrentContext = oktetoContext
		ctxOptions.Show = false
		ctxOptions.Save = true
	}

	ctxController := ContextUse{
		k8sClientProvider:    okteto.NewK8sClientProvider(),
		loginController:      login.NewLoginController(),
		oktetoClientProvider: okteto.NewOktetoClientProvider(),
	}

	if err := ctxController.UseContext(ctx, ctxOptions); err != nil {
		return err
	}

	os.Setenv(model.OktetoNamespaceEnvVar, okteto.Context().Namespace)

	if ctxOptions.Show {
		log.Information("Using %s @ %s as context", okteto.Context().Namespace, okteto.RemoveSchema(okteto.Context().Name))
	}

	if ctxOptions.IsCtxCommand {
		log.Information("Run 'okteto context update-kubeconfig' to update your kubectl credentials")
	}
	return nil
}

func getContext(ctx context.Context, ctxOptions *ContextOptions) (string, error) {
	ctxs := getContextsSelection(ctxOptions)
	oktetoContext, isOkteto, err := AskForOptions(ctx, ctxs, "A context defines the default cluster/namespace for any Okteto CLI command.\nSelect the context you want to use:")
	if err != nil {
		return "", err
	}
	ctxOptions.IsOkteto = isOkteto

	if isCreateNewContextOption(oktetoContext) {
		oktetoContext = askForOktetoURL()
		ctxOptions.IsOkteto = true
	}

	return oktetoContext, nil
}

func setSecrets(secrets []types.Secret) {
	for _, secret := range secrets {
		if os.Getenv(secret.Name) == "" {
			os.Setenv(secret.Name, secret.Value)
		}
	}
}

func (c ContextUse) getUserContext(ctx context.Context) (*types.UserContext, error) {
	client, err := c.oktetoClientProvider.NewOktetoUserClient()
	if err != nil {
		return nil, err
	}

	retries := 0
	for retries <= 3 {
		userContext, err := client.GetUserContext(ctx)

		// If userID is not on context config file we add it and save it.
		// this prevents from relogin to actual users
		if okteto.Context().UserID == "" && okteto.Context().IsOkteto {
			okteto.Context().UserID = userContext.User.ID
			if err := okteto.WriteOktetoContextConfig(); err != nil {
				log.Infof("error updating okteto contexts: %v", err)
				return nil, fmt.Errorf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
			}
		}

		if err == nil {
			return userContext, nil
		}

		if errors.IsForbidden(err) {
			okteto.Context().Token = ""
			if err := okteto.WriteOktetoContextConfig(); err != nil {
				log.Infof("error updating okteto contexts: %v", err)
				return nil, fmt.Errorf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
			}
			return nil, fmt.Errorf(errors.ErrNotLogged, okteto.Context().Name)
		}

		log.Info(err)
		retries++
	}
	return nil, errors.ErrInternalServerError
}
