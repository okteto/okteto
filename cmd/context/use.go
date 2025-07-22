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

package context

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	personalAccessTokenURL          = "https://www.okteto.com/docs/core/credentials/personal-access-tokens/"
	suggestInstallOktetoSH          = "Don't have an Okteto instance?\n    Start by installing Okteto on your Kubernetes cluster: https://www.okteto.com/free-trial/"
	messageSuggestingCurrentContext = "Enter the URL of your Okteto instance: "
)

// Use context points okteto to a cluster.
func Use() *cobra.Command {
	ctxOptions := &Options{}
	cmd := &cobra.Command{
		Use:   "use [<url>|Kubernetes context]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#use"),
		Short: "Set the default Okteto Context",
		Long: `Set the default Okteto Context.

An Okteto Context is a group of cluster access parameters.
Each context contains a Kubernetes cluster, a user, and a namespace.
The current Okteto Context is the default cluster/namespace for any Okteto CLI command.

To set your default Okteto Context, run the ` + "`okteto context`" + ` command:

    $ okteto context

This will prompt you to select one of your existing Okteto Contexts or to create a new one.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(args) == 1 {
				ctxOptions.Context = strings.TrimSuffix(args[0], "/")
			}

			ctxOptions.IsCtxCommand = true
			ctxOptions.Save = true
			ctxOptions.CheckNamespaceAccess = ctxOptions.Namespace != ""

			err := NewContextCommand().Run(ctx, ctxOptions)
			analytics.TrackContext(err == nil)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication. Use this when scripting or if you don't want to use browser-based authentication")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "overwrite the current Okteto Namespace")
	cmd.Flags().BoolVarP(&ctxOptions.OnlyOkteto, "okteto", "", false, "only shows okteto context options")
	cmd.Flags().BoolVarP(&ctxOptions.Force, "force", "", false, "delete the corresponding configuration in the okteto context and force the user to log in again")
	if err := cmd.Flags().MarkHidden("okteto"); err != nil {
		oktetoLog.Infof("failed to mark 'okteto' flag as hidden: %s", err)
	}
	return cmd
}

func (c *Command) deleteContext(contextToDelete string, ctxStore *okteto.ContextStore) error {

	// Try both with and without schema
	delete(ctxStore.Contexts, contextToDelete)
	contextWithSchema := okteto.AddSchema(contextToDelete)
	delete(ctxStore.Contexts, contextWithSchema)
	ctxStore.CurrentContext = ""

	// Write the updated context store
	if err := c.OktetoContextWriter.Write(); err != nil {
		oktetoLog.Infof("error updating okteto contexts: %v", err)
		return fmt.Errorf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
	}
	return nil
}

func (c *Command) forceLoginIfRequested(ctxOptions *Options, ctxStore *okteto.ContextStore) error {
	if ctxOptions.Force && ctxOptions.Context != "" {
		if err := c.deleteContext(ctxOptions.Context, ctxStore); err != nil {
			return err
		}

		// Clear any cached token to force re-authentication
		ctxOptions.Token = ""
		ctxOptions.InferredToken = false
	}

	return nil
}

func (c *Command) Run(ctx context.Context, ctxOptions *Options) error {
	ctxStore := okteto.GetContextStore()
	if len(ctxStore.Contexts) == 0 {
		// if the context store has no context stored, set flag to save the
		// new one generated. This is necessary for any command other than
		// 'okteto context' because by default the option is false
		// for it.
		ctxOptions.Save = true
	}

	// if the --context and --namespace flags are set, they have priority over the env vars, and current context
	// if env vars OKTETO_CONTEXT and OKTETO_NAMESPACE are set, they have priority over the current context
	err := loadDotEnv(afero.NewOsFs(), os.Setenv, os.LookupEnv)
	if err != nil {
		oktetoLog.Warning("Failed to load .env file: %s", err)
	}

	ctxOptions.InitFromEnvVars()
	ctxOptions.InitFromContext()

	if ctxOptions.IsOkteto && isUrl(ctxOptions.Context) {
		ctxOptions.Context = strings.TrimSuffix(ctxOptions.Context, "/")
	}

	if ctxOptions.Token == "" && kubeconfig.InCluster() && !isValidCluster(ctxOptions.Context) {
		if ctxOptions.IsCtxCommand {
			return oktetoErrors.ErrTokenFlagNeeded
		}
		return oktetoErrors.UserError{
			E:    oktetoErrors.ErrTokenEnvVarNeeded,
			Hint: fmt.Sprintf("Visit %s for more information about getting your token.", personalAccessTokenURL),
		}
	}

	if ctxOptions.Context == "" {
		if !ctxOptions.IsCtxCommand && !ctxOptions.raiseNotCtxError {
			oktetoLog.Information("Okteto context is not initialized")
		}
		if ctxOptions.raiseNotCtxError {
			return oktetoErrors.ErrCtxNotSet
		}
		oktetoLog.Infof("authenticating with interactive context")
		oktetoContext, err := getContext(ctxOptions)
		if err != nil {
			return err
		}
		ctxOptions.Context = oktetoContext
		ctxStore.CurrentContext = oktetoContext
		ctxOptions.Show = false
		ctxOptions.Save = true
	}

	if err := c.forceLoginIfRequested(ctxOptions, ctxStore); err != nil {
		return err
	}

	if err := c.UseContext(ctx, ctxOptions); err != nil {
		// delete the context to force the user to log in again
		if oktetoErrors.IsNotFound(err) {
			c.deleteContext(ctxOptions.Context, ctxStore)
			return oktetoErrors.AuthFailedError{Context: ctxOptions.Context}.Error()
		}

		return err
	}

	os.Setenv(model.OktetoNamespaceEnvVar, okteto.GetContext().Namespace)
	os.Setenv(model.OktetoDomainEnvVar, okteto.GetSubdomain())

	if ctxOptions.Show {
		oktetoLog.Information("Using %s @ %s as context", okteto.GetContext().Namespace, okteto.RemoveSchema(okteto.GetContext().Name))
	}

	return nil
}

// RunStateless is the fn to use until the refactoring of the context command itself if you want to make use
// of an injected context instead of using the global context variable.
func (c *Command) RunStateless(ctx context.Context, ctxOptions *Options) (*okteto.ContextStateless, error) {
	err := c.Run(ctx, ctxOptions)
	if err != nil {
		return nil, err
	}

	cfg := okteto.GetContext().Cfg.DeepCopy()
	// Storing previous global namespace gotten after executing c.Run as it is memory, but after reading the
	// context store from path that is lost
	globalNamespace := okteto.GetContext().GlobalNamespace
	builder := okteto.GetContext().Builder
	cert := okteto.GetContext().Certificate
	name := okteto.GetContext().Name
	registry := okteto.GetContext().Registry
	token := okteto.GetContext().Token
	user := okteto.GetContext().UserID
	isInsecure := okteto.GetContext().IsInsecure
	namespace := okteto.GetContext().Namespace
	isOkteto := okteto.GetContext().IsOkteto

	oktetoContextStore := okteto.GetContextStoreFromStorePath()

	oktetoContextStateless := &okteto.ContextStateless{
		Store: oktetoContextStore,
	}

	oktetoContextStateless.SetCurrentCfg(cfg)
	oktetoContextStateless.SetGlobalNamespace(globalNamespace)
	oktetoContextStateless.SetCurrentBuilder(builder)
	oktetoContextStateless.SetCurrentCertStr(cert)
	oktetoContextStateless.SetCurrentName(name)
	oktetoContextStateless.SetCurrentRegistry(registry)
	oktetoContextStateless.SetCurrentToken(token)
	oktetoContextStateless.SetCurrentUser(user)
	oktetoContextStateless.SetInsecure(isInsecure)
	oktetoContextStateless.SetNamespace(namespace)
	oktetoContextStateless.SetOktetoCluster(isOkteto)

	return oktetoContextStateless, nil

}

func getContext(ctxOptions *Options) (string, error) {
	ctxs := getAvailableContexts(ctxOptions)

	var oktetoContext string
	var err error
	if len(ctxs) > 0 {
		ctxs = append(ctxs, utils.SelectorItem{
			Label:  "",
			Enable: false,
		})
		ctxs = append(ctxs, utils.SelectorItem{
			Name:   newOEOption,
			Label:  newOEOption,
			Enable: true,
		})

		initialPosition := getInitialPosition(ctxs)
		selector := utils.NewOktetoSelector("A context defines the default Okteto instance or cluster for any Okteto CLI command.\nSelect the context you want to use:", "Option")
		oktetoContext, err = selector.AskForOptionsOkteto(ctxs, initialPosition)
		if err != nil {
			return "", err
		}
		if isCreateNewContextOption(oktetoContext) {
			ctxStore := okteto.GetContextStore()
			clusterURL := ""
			if oCtx, ok := ctxStore.Contexts[ctxStore.CurrentContext]; ok && oCtx.IsOkteto {
				clusterURL = ctxStore.CurrentContext
			}
			question := fmt.Sprintf("%s[%s]: ", messageSuggestingCurrentContext, clusterURL)
			oktetoContext, err = askForOktetoURL(question)
			if err != nil {
				return "", err
			}
			ctxOptions.IsOkteto = true
		} else {
			ctxOptions.IsOkteto = okteto.IsOktetoContext(oktetoContext)
		}
	} else {

		oktetoLog.Information(suggestInstallOktetoSH)
		oktetoContext, err = askForOktetoURL(messageSuggestingCurrentContext)
		if err != nil {
			return "", err
		}
		ctxOptions.IsOkteto = true
	}

	return oktetoContext, nil
}

func exportPlatformVariablesToEnv(variables []env.Var) {
	for _, v := range variables {
		value, exists := os.LookupEnv(v.Name)
		if exists {
			if value != v.Value {
				oktetoLog.Warning("Okteto Variable '%s' is overridden by a local environment variable with the same name", v.Name)
			}
			oktetoLog.AddMaskedWord(value)
			continue
		}
		os.Setenv(v.Name, v.Value)
		oktetoLog.AddMaskedWord(v.Value)
	}
}
