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

// Context points okteto to a cluster.
func Context() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Use:     "context [url|k8s-context]",
		Aliases: []string{"ctx"},
		Args:    utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#context"),
		Short:   "Manage your okteto context",
		Long: `Manage your okteto context

A context is a group of cluster access parameters. Each context contains a Kubernetes cluster, a user, and a namespace.
The current context is the default cluster/namespace for any Okteto CLI command.

If you want to log into an Okteto Enterprise instance, specify a URL. For example, run:

    $ okteto context https://cloud.okteto.com

to configure your context to access Okteto Cloud.

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
			if len(args) == 1 {
				ctxOptions.Context = args[0]
			}

			ctxOptions.isCtxCommand = true
			ctxOptions.Save = true
			err := Run(ctx, ctxOptions)
			analytics.TrackContext(err == nil)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.AddCommand(List())
	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "namespace of your okteto context")
	cmd.Flags().StringVarP(&ctxOptions.Builder, "builder", "b", "", "url of the builder service")
	cmd.Flags().BoolVarP(&ctxOptions.OnlyOkteto, "okteto", "", false, "only shows okteto cluster options")
	return cmd
}

func Run(ctx context.Context, ctxOptions *ContextOptions) error {
	ctxStore := okteto.ContextStore()
	ctxOptions.initFromContext()
	ctxOptions.initFromEnvVars()

	if ctxOptions.Token == "" && ctxOptions.Context == "" && kubeconfig.InCluster() {
		return errors.ErrTokenFlagNeeded
	}

	if ctxOptions.Token == "" && !okteto.IsOktetoURL(ctxOptions.Context) && kubeconfig.InCluster() {
		return errors.ErrTokenFlagNeeded
	}

	//K8sContextToOktetoUrl translates k8s contexts like cloud_okteto_com to hettps://cloud.okteto.com
	ctxOptions.Context = okteto.K8sContextToOktetoUrl(strings.TrimSuffix(ctxOptions.Context, "/"))

	if ctxOptions.Context == "" {
		log.Infof("authenticating with interactive context")
		oktetoContext, err := getContext(ctxOptions)
		if err != nil {
			return err
		}
		ctxOptions.Context = oktetoContext
		ctxStore.CurrentContext = oktetoContext
		ctxOptions.Save = true
		ctxOptions.Show = false
	}

	if okCtx, ok := ctxStore.Contexts[ctxOptions.Context]; !ok {
		ctxStore.Contexts[ctxOptions.Context] = &okteto.OktetoContext{Name: ctxOptions.Context}
	} else if ctxOptions.Token == "" {
		//this is to avoid login with the browser again if we already have a valid token
		ctxOptions.Token = okCtx.Token
	}
	ctxStore.CurrentContext = ctxOptions.Context

	if okteto.IsOktetoURL(ctxOptions.Context) {
		if err := initOktetoContext(ctx, ctxOptions); err != nil {
			return err
		}
	} else {
		if err := initKubernetesContext(ctxOptions); err != nil {
			return err
		}
	}

	if ctxOptions.Save {
		if err := okteto.WriteOktetoContextConfig(); err != nil {
			return err
		}
		if err := kubeconfig.Write(okteto.Context().Cfg, config.GetKubeconfigPath()); err != nil {
			return err
		}
		k8sContext := ctxOptions.Context
		if okteto.IsOktetoURL(k8sContext) {
			k8sContext = okteto.UrlToKubernetesContext(k8sContext)
		}
		log.Information("Current kubernetes context '%s/%s' in '%s'", k8sContext, ctxOptions.Namespace, config.GetKubeconfigPath())
	}

	os.Setenv("OKTETO_NAMESPACE", okteto.Context().Namespace)

	if ctxOptions.Show {
		log.Information("Using %s @ %s as context", okteto.Context().Namespace, strings.TrimPrefix(okteto.Context().Name, "https://"))
	}

	return nil
}

func getContext(ctxOptions *ContextOptions) (string, error) {
	ctxs := getContextsSelection(ctxOptions)
	oktetoContext, err := AskForOptions(ctxs, "A context defines the default cluster/namespace for any Okteto CLI command.\nSelect the context you want to use:")
	if err != nil {
		return "", err
	}

	if isOktetoCluster(oktetoContext) {
		oktetoContext = getOktetoClusterUrl(oktetoContext)
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
	okteto.AddOktetoContext(ctxOptions.Context, &userContext.User, ctxOptions.Namespace)
	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		cfg = kubeconfig.Create()
	}
	okteto.AddOktetoCredentialsToCfg(cfg, &userContext.Credentials, ctxOptions.Namespace, userContext.User.ID, okteto.UrlToKubernetesContext(okteto.Context().Name))
	okteto.Context().Cfg = cfg

	setSecrets(userContext.Secrets)

	os.Setenv("OKTETO_USERNAME", okteto.Context().Username)

	return nil
}

func initKubernetesContext(ctxOptions *ContextOptions) error {
	if !isValidCluster(ctxOptions.Context) {
		return errors.UserError{
			E:    fmt.Errorf(errors.ErrInvalidContext, ctxOptions.Context),
			Hint: fmt.Sprintf("Valid Kubernetes contexts are:\n      %s", strings.Join(getKubernetesContextList(), "\n      ")),
		}
	}

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

	return nil
}
