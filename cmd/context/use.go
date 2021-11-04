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
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Use context points okteto to a cluster.
func Use() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Use:   "use [<url>|Kubernetes context]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#context"),
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

			ctxOptions.isCtxCommand = true
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
		if !ctxOptions.isCtxCommand {
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
	}

	if err := UseContext(ctx, ctxOptions); err != nil {
		return err
	}

	os.Setenv("OKTETO_NAMESPACE", okteto.Context().Namespace)

	if ctxOptions.Show {
		log.Information("Using %s @ %s as context", okteto.Context().Namespace, okteto.RemoveSchema(okteto.Context().Name))
	}

	if ctxOptions.isCtxCommand {
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
	ctxOptions.isOkteto = isOkteto

	if isCreateNewContextOption(oktetoContext) {
		oktetoContext = askForOktetoURL()
		ctxOptions.isOkteto = true
	}

	return oktetoContext, nil
}

func setSecrets(secrets []okteto.Secret) {
	for _, secret := range secrets {
		os.Setenv(secret.Name, secret.Value)
	}
}

func getUserContext(ctx context.Context) (*okteto.UserContext, error) {
	client, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	retries := 0
	for retries <= 3 {
		userContext, err := client.GetUserContext(ctx)
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

func initOktetoContext(ctx context.Context, ctxOptions *ContextOptions) error {
	user, err := login.AuthenticateToOktetoCluster(ctx, ctxOptions.Context, ctxOptions.Token)
	if err != nil {
		return err
	}
	ctxOptions.Token = user.Token
	okteto.Context().Token = user.Token

	userContext, err := getUserContext(ctx)
	if err != nil {
		return err
	}
	if ctxOptions.Namespace == "" {
		ctxOptions.Namespace = userContext.User.Namespace
	}
	hasAccess, err := utils.HasAccessToNamespace(ctx, ctxOptions.Namespace)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.UserError{E: fmt.Errorf("namespace '%s' not found on context '%s'", ctxOptions.Namespace, ctxOptions.Context),
			Hint: "Please verify that the namespace exists and that you have access to it.",
		}
	}

	okteto.AddOktetoContext(ctxOptions.Context, &userContext.User, ctxOptions.Namespace)
	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		cfg = kubeconfig.Create()
	}
	okteto.AddOktetoCredentialsToCfg(cfg, &userContext.Credentials, ctxOptions.Namespace, userContext.User.ID, okteto.Context().Name)
	okteto.Context().Cfg = cfg
	okteto.Context().IsOkteto = true

	setSecrets(userContext.Secrets)

	os.Setenv("OKTETO_USERNAME", okteto.Context().Username)

	return nil
}

func initKubernetesContext(ctxOptions *ContextOptions) error {
	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		return fmt.Errorf(errors.ErrKubernetesContextNotFound, ctxOptions.Context, config.GetKubeconfigPath())
	}
	kubeCtx, ok := cfg.Contexts[ctxOptions.Context]
	if !ok {
		return fmt.Errorf(errors.ErrKubernetesContextNotFound, ctxOptions.Context, config.GetKubeconfigPath())
	}
	cfg.CurrentContext = ctxOptions.Context
	if ctxOptions.Namespace != "" {
		cfg.Contexts[ctxOptions.Context].Namespace = ctxOptions.Namespace
	} else {
		if cfg.Contexts[ctxOptions.Context].Namespace == "" {
			cfg.Contexts[ctxOptions.Context].Namespace = "default"
		}
		ctxOptions.Namespace = cfg.Contexts[ctxOptions.Context].Namespace
	}

	okteto.AddKubernetesContext(ctxOptions.Context, ctxOptions.Namespace, ctxOptions.Builder)

	kubeCtx.Namespace = okteto.Context().Namespace
	cfg.CurrentContext = okteto.Context().Name
	okteto.Context().Cfg = cfg
	okteto.Context().IsOkteto = false

	return nil
}
