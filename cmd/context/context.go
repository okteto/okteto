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
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
The current context is the cluster that is currently the default for the Okteto CLI. All "okteto" commands run against that cluster.

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
			if ctxOptions.Token == "" && client.InCluster() {
				return errors.ErrTokenFlagNeeded
			}

			apiToken := os.Getenv("OKTETO_TOKEN")
			if ctxOptions.Token == "" {
				ctxOptions.Token = apiToken
			}

			if err := okteto.InitContext(ctx, ctxOptions.Token); err != nil {
				if err != errors.ErrNoActiveOktetoContexts {
					return err
				}
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
		user, err := login.AuthenticateToOktetoCluster(ctx, oktetoContext, ctxOptions.Token)
		if err != nil {
			return err
		}

		octxStore := okteto.ContextStore()
		octxStore.CurrentContext = oktetoContext
		octxStore.Contexts[oktetoContext] = &okteto.OktetoContext{
			Name:  oktetoContext,
			Token: user.Token,
		}

		oktetoClient, err := okteto.NewOktetoClient()
		if err != nil {
			return err
		}
		cred, err := oktetoClient.GetCredentials(ctx)
		if err != nil {
			return err
		}

		if ctxOptions.Namespace == "" {
			ctxOptions.Namespace = cred.Namespace
		}

		if err := okteto.SetKubeContext(cred, kubeconfigFile, ctxOptions.Namespace, user.ID, okteto.UrlToContext(oktetoContext)); err != nil {
			return fmt.Errorf("error updating kubernetes context: %v", err)
		}
		log.Success("Updated kubernetes context: %s", okteto.UrlToContext(oktetoContext))

		cfg := client.GetKubeconfig(kubeconfigFile)
		if err := okteto.SaveOktetoClusterContext(oktetoContext, user, ctxOptions.Namespace, cfg); err != nil {
			return fmt.Errorf("error configuring okteto context: %v", err)
		}

		return nil
	}

	if !isValidCluster(oktetoContext) {
		return errors.UserError{
			E:    fmt.Errorf(errors.ErrInvalidContext, oktetoContext),
			Hint: fmt.Sprintf("Valid Kubernetes contexts are:\n      %s", strings.Join(getKubernetesContextList(), "\n      ")),
		}
	}
	cfg := client.GetKubeconfig(kubeconfigFile)
	cfg.CurrentContext = oktetoContext
	if ctxOptions.Namespace != "" {
		cfg.Contexts[oktetoContext].Namespace = ctxOptions.Namespace
	} else {
		ctxOptions.Namespace = client.GetCurrentNamespace(kubeconfigFile)
	}
	if err := client.WriteKubeconfig(cfg, kubeconfigFile); err != nil {
		return err
	}
	if err := okteto.SaveKubernetesClusterContext(oktetoContext, ctxOptions.Namespace, cfg, ctxOptions.Builder); err != nil {
		return err
	}
	log.Success("Updated kubernetes context: %s", oktetoContext)
	return nil
}

func getContext(ctxOptions *ContextOptions) (string, error) {
	ctxs := getContextsSelection(ctxOptions)
	oktetoContext, err := AskForOptions(ctxs, "Select the context you want to activate:")
	if err != nil {
		return "", err
	}

	if isOktetoCluster(oktetoContext) {
		oktetoContext = getOktetoClusterUrl(oktetoContext)
	}

	return oktetoContext, nil
}

func Init(ctx context.Context) error {
	if err := okteto.InitContext(ctx, ""); err != nil {
		if err != errors.ErrNoActiveOktetoContexts {
			return err
		}
		okCtx := Context()
		okCtx.Flags().Set("okteto", "true")
		if err := okCtx.RunE(nil, nil); err != nil {
			return err
		}
	}
	if okteto.IsOktetoContext() {

		secretsAndKubeCredentials, err := getSecretsAndCredentials(ctx)
		if err != nil {
			return err
		}
		if err := updateContext(secretsAndKubeCredentials.Credentials); err != nil {
			log.Info(err)
		}
		setSecrets(secretsAndKubeCredentials.Secrets)

		os.Setenv("OKTETO_USERNAME", okteto.Context().Username)
		os.Setenv("OKTETO_NAMESPACE", okteto.Context().Namespace)
	}

	return nil
}

func setSecrets(secrets []okteto.Secret) {
	for _, secret := range secrets {
		os.Setenv(secret.Name, secret.Value)
	}
}

func updateContext(cred okteto.Credential) error {
	octx := okteto.Context()
	kubeconfigFile := config.GetKubeconfigPath()
	cfg := client.GetKubeconfig(kubeconfigFile)
	u := octx.ToUser()

	clusterName := okteto.UrlToContext(octx.Name)
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}

	cluster.CertificateAuthorityData = []byte(cred.Certificate)
	cluster.Server = cred.Server

	return okteto.SaveOktetoClusterContext(okteto.Context().Name, u, okteto.Context().Namespace, cfg)
}

func getSecretsAndCredentials(ctx context.Context) (*okteto.SecretsAndCredentialToken, error) {
	client, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	retries := 3
	var secretsAndKubeCredentials *okteto.SecretsAndCredentialToken
	for retries > 0 {
		secretsAndKubeCredentials, err = client.GetSecretsAndKubeCredentials(ctx)
		if err != nil {
			retries -= 1
		}
		if err != nil {
			log.Info(err)
			retries -= 1
		} else {
			break
		}
	}
	return secretsAndKubeCredentials, err
}
