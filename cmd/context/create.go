// Copyright 2022 The Okteto Authors
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
	"net/url"
	"os"
	"strings"

	"github.com/compose-spec/godotenv"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// ContextCommand has the dependencies to run a ctxCommand
type ContextCommand struct {
	K8sClientProvider    okteto.K8sClientProvider
	LoginController      login.LoginInterface
	OktetoClientProvider types.OktetoClientProvider

	OktetoContextWriter okteto.ContextConfigWriterInterface
}

// NewContextCommand creates a new ContextCommand
func NewContextCommand() *ContextCommand {
	return &ContextCommand{
		K8sClientProvider:    okteto.NewK8sClientProvider(),
		LoginController:      login.NewLoginController(),
		OktetoClientProvider: okteto.NewOktetoClientProvider(),
		OktetoContextWriter:  okteto.NewContextConfigWriter(),
	}
}

// Create adds a new cluster to okteto context
func CreateCMD() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "create [cluster-url]",
		Args:   utils.ExactArgsAccepted(1, "https://okteto.com/docs/reference/cli/#create"),
		Short:  "Add a context",
		Long: `Add a context

A context is a group of cluster access parameters. Each context contains a Kubernetes cluster, a user, and a namespace.
The current context is the default cluster/namespace for any Okteto CLI command.

You need to specify an Okteto URL. For example, run:

	$ okteto context create https://cloud.okteto.com

to configure your context to access Okteto Cloud.

Your browser will ask for your authentication to retrieve your API token.

If you need to automate authentication or if you don't want to use browser-based authentication, use the "--token" parameter:

	$ okteto context create https://cloud.okteto.com --token ${OKTETO_TOKEN}
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto context create' is deprecated in favor of 'okteto context use', and will be removed in version 1.16")
			ctx := context.Background()

			ctxOptions.Context = args[0]
			ctxOptions.Context = okteto.AddSchema(ctxOptions.Context)
			ctxOptions.Context = strings.TrimSuffix(ctxOptions.Context, "/")
			ctxOptions.IsOkteto = true
			ctxOptions.IsCtxCommand = true
			ctxOptions.Show = false
			ctxOptions.Save = true

			err := NewContextCommand().UseContext(ctx, ctxOptions)
			analytics.TrackContext(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "namespace of your okteto context")
	cmd.Flags().StringVarP(&ctxOptions.Builder, "builder", "b", "", "url of the builder service")
	return cmd
}

func (c *ContextCommand) UseContext(ctx context.Context, ctxOptions *ContextOptions) error {
	created := false

	ctxStore := okteto.ContextStore()
	if okCtx, ok := ctxStore.Contexts[ctxOptions.Context]; ok && okCtx.IsOkteto {
		ctxOptions.IsOkteto = true
	}

	if okCtx, ok := ctxStore.Contexts[okteto.AddSchema(ctxOptions.Context)]; ok && okCtx.IsOkteto {
		ctxOptions.Context = okteto.AddSchema(ctxOptions.Context)
		ctxOptions.IsOkteto = true
	}

	if ctxOptions.Context == okteto.CloudURL {
		ctxOptions.IsOkteto = true
	}

	if !ctxOptions.IsOkteto {

		if isUrl(ctxOptions.Context) {
			ctxOptions.Context = strings.TrimSuffix(ctxOptions.Context, "/")
			ctxOptions.IsOkteto = true
		} else {
			if !isValidCluster(ctxOptions.Context) {
				return oktetoErrors.UserError{E: fmt.Errorf("invalid okteto context '%s'", ctxOptions.Context),
					Hint: "Please run 'okteto context' to select one context"}
			}
			transformedCtx := okteto.K8sContextToOktetoUrl(ctx, ctxOptions.Context, ctxOptions.Namespace, c.K8sClientProvider)
			if transformedCtx != ctxOptions.Context {
				ctxOptions.Context = transformedCtx
				ctxOptions.IsOkteto = true
			}
		}
	}

	if okCtx, ok := ctxStore.Contexts[ctxOptions.Context]; !ok {
		ctxStore.Contexts[ctxOptions.Context] = &okteto.OktetoContext{Name: ctxOptions.Context}
		created = true
	} else if ctxOptions.Token == "" {
		//this is to avoid login with the browser again if we already have a valid token
		ctxOptions.Token = okCtx.Token
		if ctxOptions.Builder == "" && okCtx.Builder != "" {
			ctxOptions.Builder = okCtx.Builder
		}
	}

	ctxStore.CurrentContext = ctxOptions.Context
	c.initEnvVars()

	if ctxOptions.IsOkteto {
		if err := c.initOktetoContext(ctx, ctxOptions); err != nil {
			return err
		}
	} else {
		if err := c.initKubernetesContext(ctxOptions); err != nil {
			return err
		}
	}
	if ctxOptions.IsOkteto && ctxOptions.Save {
		okClient, err := c.OktetoClientProvider.Provide()
		if err != nil {
			return err
		}
		hasAccess, err := utils.HasAccessToNamespace(ctx, ctxOptions.Namespace, okClient)
		if err != nil {
			return err
		}
		if !hasAccess {
			return oktetoErrors.UserError{E: fmt.Errorf("namespace '%s' not found on context '%s'", ctxOptions.Namespace, ctxOptions.Context),
				Hint: "Please verify that the namespace exists and that you have access to it.",
			}
		}
	}
	if ctxOptions.Save {
		if err := c.OktetoContextWriter.Write(); err != nil {
			return err
		}
	}
	if created && ctxOptions.IsOkteto {
		oktetoLog.Success("Context '%s' created", okteto.RemoveSchema(ctxOptions.Context))
	}

	if ctxOptions.IsCtxCommand {
		oktetoLog.Success("Using context %s @ %s", okteto.Context().Namespace, okteto.RemoveSchema(ctxStore.CurrentContext))
	}

	return nil
}

func (c *ContextCommand) initOktetoContext(ctx context.Context, ctxOptions *ContextOptions) error {
	user, err := c.LoginController.AuthenticateToOktetoCluster(ctx, ctxOptions.Context, ctxOptions.Token)
	if err != nil {
		return err
	}
	ctxOptions.Token = user.Token
	okteto.Context().Token = user.Token

	userContext, err := c.getUserContext(ctx)
	if err != nil {
		return err
	}
	if ctxOptions.Namespace == "" {
		ctxOptions.Namespace = userContext.User.Namespace
	}
	okteto.AddOktetoContext(ctxOptions.Context, &userContext.User, ctxOptions.Namespace, userContext.User.Namespace)
	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		cfg = kubeconfig.Create()
	}
	okteto.AddOktetoCredentialsToCfg(cfg, &userContext.Credentials, ctxOptions.Namespace, userContext.User.ID, okteto.Context().Name)
	okteto.Context().Cfg = cfg
	okteto.Context().IsOkteto = true

	setSecrets(userContext.Secrets)

	os.Setenv(model.OktetoUserNameEnvVar, okteto.Context().Username)

	return nil
}

func (*ContextCommand) initKubernetesContext(ctxOptions *ContextOptions) error {
	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		return fmt.Errorf(oktetoErrors.ErrKubernetesContextNotFound, ctxOptions.Context, config.GetKubeconfigPath())
	}
	kubeCtx, ok := cfg.Contexts[ctxOptions.Context]
	if !ok {
		return fmt.Errorf(oktetoErrors.ErrKubernetesContextNotFound, ctxOptions.Context, config.GetKubeconfigPath())
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

func (c ContextCommand) getUserContext(ctx context.Context) (*types.UserContext, error) {
	client, err := c.OktetoClientProvider.Provide()
	if err != nil {
		return nil, err
	}

	retries := 0
	for retries <= 3 {
		userContext, err := client.User().GetContext(ctx)

		if err != nil && oktetoErrors.IsForbidden(err) {
			okteto.Context().Token = ""
			if err := c.OktetoContextWriter.Write(); err != nil {
				oktetoLog.Infof("error updating okteto contexts: %v", err)
				return nil, fmt.Errorf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
			}
			return nil, fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.Context().Name)
		}
		if err != nil {
			oktetoLog.Info(err)
			retries++
			continue
		}

		// If userID is not on context config file we add it and save it.
		// this prevents from relogin to actual users
		if okteto.Context().UserID == "" && okteto.Context().IsOkteto {
			okteto.Context().UserID = userContext.User.ID
			if err := c.OktetoContextWriter.Write(); err != nil {
				oktetoLog.Infof("error updating okteto contexts: %v", err)
				return nil, fmt.Errorf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
			}
		}
		return userContext, nil
	}
	return nil, oktetoErrors.ErrInternalServerError
}

func (*ContextCommand) initEnvVars() {
	if model.FileExists(".env") {
		if err := godotenv.Load(); err != nil {
			oktetoLog.Infof("error loading .env file: %s", err.Error())
		}
	}
}

func isUrl(u string) bool {
	parsedUrl, err := url.Parse(u)
	if err != nil {
		oktetoLog.Infof("could not parse %s", u)
		return false
	}
	return parsedUrl.Scheme != "" && parsedUrl.Host != ""
}
