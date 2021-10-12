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
	"github.com/spf13/cobra"
)

type ContextOptions struct {
	Token      string
	Namespace  string
	Builder    string
	OnlyOkteto bool
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
			if ctxOptions.Token == "" && kubeconfig.InCluster() {
				return errors.ErrTokenFlagNeeded
			}

			apiToken := os.Getenv("OKTETO_TOKEN")
			if ctxOptions.Token == "" {
				ctxOptions.Token = apiToken
			}

			var err error
			oktetoContext := os.Getenv("OKTETO_URL")
			if oktetoContext == "" && ctxOptions.Token != "" {
				oktetoContext = okteto.CloudURL
			}

			if len(args) == 0 {
				if oktetoContext != "" {
					log.Infof("authenticating with OKTETO_URL")
				} else {
					log.Infof("authenticating with interactive context")
					oktetoContext, err = getContext(ctxOptions)
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

func runContext(ctx context.Context, oktetoContext string, ctxOptions *ContextOptions) error {

	kubeconfigFile := config.GetKubeconfigPath()

	if okteto.IsOktetoURL(oktetoContext) {
		octxStore := okteto.ContextStore()
		octxStore.CurrentContext = oktetoContext
		if _, ok := octxStore.Contexts[oktetoContext]; !ok {
			octxStore.Contexts[oktetoContext] = &okteto.OktetoContext{
				Name: oktetoContext,
			}
		}

		user, err := login.AuthenticateToOktetoCluster(ctx, oktetoContext, ctxOptions.Token)
		if err != nil {
			return err
		}
		octxStore.Contexts[oktetoContext].Token = user.Token

		if ctxOptions.Namespace == "" {
			ctxOptions.Namespace = user.Namespace
		}

		okteto.AddOktetoContext(oktetoContext, user, ctxOptions.Namespace)

		oktetoClient, err := okteto.NewOktetoClient()
		if err != nil {
			return err
		}
		cred, err := oktetoClient.GetCredentials(ctx)
		if err != nil {
			return err
		}
		if err := okteto.WriteKubeconfig(cred, kubeconfigFile, ctxOptions.Namespace, user.ID, okteto.UrlToKubernetesContext(oktetoContext)); err != nil {
			return fmt.Errorf("error updating kubernetes context: %v", err)
		}
		if err := okteto.WriteOktetoContextConfig(); err != nil {
			return fmt.Errorf("error configuring okteto context: %v", err)
		}
		log.Information("Current kubernetes context '%s' in '%s'", okteto.UrlToKubernetesContext(oktetoContext), kubeconfigFile)

		return nil
	}

	if !isValidCluster(oktetoContext) {
		return errors.UserError{
			E:    fmt.Errorf(errors.ErrInvalidContext, oktetoContext),
			Hint: fmt.Sprintf("Valid Kubernetes contexts are:\n      %s", strings.Join(getKubernetesContextList(), "\n      ")),
		}
	}
	cfg := kubeconfig.Get(kubeconfigFile)
	if cfg == nil {
		return fmt.Errorf(errors.ErrKubernetesContextNotFound, oktetoContext, config.GetKubeconfigPath())
	}
	cfg.CurrentContext = oktetoContext
	if ctxOptions.Namespace != "" {
		cfg.Contexts[oktetoContext].Namespace = ctxOptions.Namespace
	} else {
		if cfg.Contexts[oktetoContext].Namespace == "" {
			cfg.Contexts[oktetoContext].Namespace = "default"
		}
		ctxOptions.Namespace = cfg.Contexts[oktetoContext].Namespace
	}
	okteto.AddKubernetesContext(oktetoContext, ctxOptions.Namespace, ctxOptions.Builder)

	if err := okteto.WriteOktetoContextConfig(); err != nil {
		return err
	}
	if err := kubeconfig.Write(cfg, kubeconfigFile); err != nil {
		return err
	}
	log.Information("Current kubernetes context '%s' in '%s'", oktetoContext, kubeconfigFile)

	return nil
}

func getContext(ctxOptions *ContextOptions) (string, error) {
	ctxs := getContextsSelection(ctxOptions)
	oktetoContext, err := AskForOptions(ctxs, "A context defines the default cluster/namespace for any Okteto CLI command. Select the context you want to use:")
	if err != nil {
		return "", err
	}

	if isOktetoCluster(oktetoContext) {
		oktetoContext = getOktetoClusterUrl(oktetoContext)
	}

	return oktetoContext, nil
}

func Init(ctx context.Context, ctxResource *model.ContextResource) error {
	okteto.ContextWithOktetoEnvVars(ctx, ctxResource)

	ctxStore := okteto.ContextStore()
	if ctxResource.Context != "" {
		if okteto.IsOktetoURL(ctxResource.Context) {
			if _, ok := ctxStore.Contexts[ctxResource.Context]; !ok {
				return fmt.Errorf(errors.ErrOktetoContextNotFound, ctxResource.Context, ctxResource.Context)
			}
		} else {
			for name := range ctxStore.Contexts {
				if okteto.IsOktetoURL(name) && okteto.UrlToKubernetesContext(name) == ctxResource.Context {
					ctxResource.Context = name
					break
				}
			}

			if !okteto.IsOktetoURL(ctxResource.Context) {
				if _, ok := ctxStore.Contexts[ctxResource.Context]; !ok {
					kubeconfigFile := config.GetKubeconfigPath()
					cfg := kubeconfig.Get(kubeconfigFile)
					if cfg == nil {
						return fmt.Errorf(errors.ErrKubernetesContextNotFound, ctxResource.Context, config.GetKubeconfigPath())
					}
					if _, ok := cfg.Contexts[ctxResource.Context]; !ok {
						return fmt.Errorf(errors.ErrKubernetesContextNotFound, ctxResource.Context, kubeconfigFile)
					}
					if ctxResource.Namespace == "" {
						ctxResource.Namespace = cfg.Contexts[ctxResource.Context].Namespace
					}
					if ctxResource.Namespace == "" {
						ctxResource.Namespace = "default"
					}
					okteto.AddKubernetesContext(ctxResource.Context, ctxResource.Namespace, "")
				}
			}
		}
		ctxStore.CurrentContext = ctxResource.Context
	}

	if ctxResource.Namespace != "" && ctxStore.CurrentContext != "" {
		ctxStore.Contexts[ctxStore.CurrentContext].Namespace = ctxResource.Namespace
	}

	showCurrentContext := true
	if ctxStore.CurrentContext == "" {
		showCurrentContext = false
		okCtx := Context()
		if err := okCtx.RunE(nil, nil); err != nil {
			return err
		}
	}

	if okteto.IsOkteto() {
		userContext, err := getUserContext(ctx)
		if err != nil {
			return err
		}

		if ctxResource.Namespace == "" {
			ctxResource.Namespace = okteto.Context().Namespace
		}

		if ctxResource.Namespace == "" {
			ctxResource.Namespace = userContext.Credentials.Namespace
		}

		okteto.AddOktetoContext(okteto.Context().Name, &userContext.User, ctxResource.Namespace)
		cfg := kubeconfig.Create()
		okteto.AddOktetoCredentialsToCfg(cfg, &userContext.Credentials, ctxResource.Namespace, userContext.User.ID, okteto.UrlToKubernetesContext(okteto.Context().Name))
		okteto.Context().Cfg = cfg

		setSecrets(userContext.Secrets)

		os.Setenv("OKTETO_USERNAME", okteto.Context().Username)

	} else {
		cfg := kubeconfig.Get(config.GetKubeconfigPath())
		if cfg == nil {
			return fmt.Errorf(errors.ErrKubernetesContextNotFound, okteto.Context().Name, config.GetKubeconfigPath())
		}
		kubeCtx, ok := cfg.Contexts[okteto.Context().Name]
		if !ok {
			return fmt.Errorf(errors.ErrKubernetesContextNotFound, okteto.Context().Name, config.GetKubeconfigPath())
		}
		kubeCtx.Namespace = okteto.Context().Namespace
		cfg.CurrentContext = okteto.Context().Name
		okteto.Context().Cfg = cfg
	}

	os.Setenv("OKTETO_NAMESPACE", okteto.Context().Namespace)

	if showCurrentContext {
		log.Information("Context: %s", okteto.Context().Name)
		log.Information("Namespace: %s", okteto.Context().Namespace)
	}

	return nil
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
			return nil, fmt.Errorf(errors.ErrNotLogged, okteto.Context().Name)
		}

		log.Info(err)
		retries++
	}
	return nil, errors.ErrInternalServerError
}
