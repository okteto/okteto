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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/compose-spec/godotenv"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// oktetoClientProvider provides an okteto client ready to use or fail
type oktetoClientProvider interface {
	Provide(...okteto.Option) (types.OktetoInterface, error)
}

type kubeconfigTokenController interface {
	updateOktetoContextToken(*types.UserContext) error
}

// Command has the dependencies to run a ctxCommand
type Command struct {
	K8sClientProvider    okteto.K8sClientProvider
	LoginController      login.Interface
	OktetoClientProvider oktetoClientProvider

	kubetokenController kubeconfigTokenController
	OktetoContextWriter okteto.ContextConfigWriterInterface
}

type ctxCmdOption func(*Command)

func withKubeTokenController(k kubeconfigTokenController) ctxCmdOption {
	return func(c *Command) {
		c.kubetokenController = k
	}
}

// NewContextCommand creates a new Command
func NewContextCommand(ctxCmdOption ...ctxCmdOption) *Command {
	cfg := &Command{
		K8sClientProvider:    okteto.NewK8sClientProvider(),
		LoginController:      login.NewLoginController(),
		OktetoClientProvider: okteto.NewOktetoClientProvider(),
		OktetoContextWriter:  okteto.NewContextConfigWriter(),
	}
	if env.LoadBoolean(OktetoUseStaticKubetokenEnvVar) {
		cfg.kubetokenController = newStaticKubetokenController()
	} else {
		cfg.kubetokenController = newDynamicKubetokenController(cfg.OktetoClientProvider)
	}
	for _, o := range ctxCmdOption {
		o(cfg)
	}
	return cfg
}

// CreateCMD adds a new cluster to okteto context
func CreateCMD() *cobra.Command {
	ctxOptions := &Options{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "create [cluster-url]",
		Args:   utils.ExactArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#context"),
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
			oktetoLog.Warning("'okteto context create' is deprecated in favor of 'okteto context use', and will be removed in a future version")
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

func (c *Command) UseContext(ctx context.Context, ctxOptions *Options) error {
	created := false

	ctxStore := okteto.GetContextStore()
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
		ctxStore.Contexts[ctxOptions.Context] = &okteto.Context{Name: ctxOptions.Context}
		created = true
	} else if ctxOptions.Token == "" {
		// this is to avoid login with the browser again if we already have a valid token
		ctxOptions.Token = okCtx.Token
		ctxOptions.InferredToken = true
		if ctxOptions.Builder == "" && okCtx.Builder != "" {
			ctxOptions.Builder = okCtx.Builder
		}
		if ctxOptions.Namespace == "" {
			ctxOptions.Namespace = ctxStore.Contexts[ctxOptions.Context].Namespace
		}

	}

	ctxStore.CurrentContext = ctxOptions.Context
	err := c.loadDotEnv(afero.NewOsFs(), os.Setenv)
	if err != nil {
		oktetoLog.Warning("Failed to load .env file: %s", err)
	}

	if ctxOptions.IsOkteto {
		if err := c.initOktetoContext(ctx, ctxOptions); err != nil {
			return err
		}
	} else {
		if err := c.initKubernetesContext(ctxOptions); err != nil {
			return err
		}
	}

	if ctxOptions.Save {
		hasAccess, err := hasAccessToNamespace(ctx, c, ctxOptions)
		if err != nil {
			return err
		}

		if !hasAccess {
			if ctxOptions.CheckNamespaceAccess {
				return oktetoErrors.UserError{
					E:    fmt.Errorf("namespace '%s' not found on context '%s'", ctxOptions.Namespace, ctxOptions.Context),
					Hint: "Please verify that the namespace exists and that you have access to it.",
				}
			}

			// if using a new context, our cached namespace may have been removed
			// so swap over to the personal namespace instead of erroring
			oktetoLog.Warning(
				"No access to namespace '%s' switching to personal namespace '%s'",
				ctxOptions.Namespace,
				okteto.GetContext().PersonalNamespace,
			)
			currentCtx := ctxStore.Contexts[ctxOptions.Context]
			currentCtx.Namespace = currentCtx.PersonalNamespace
		}

		currentCtx := ctxStore.Contexts[ctxOptions.Context]
		currentCtx.IsStoredAsInsecure = okteto.IsInsecureSkipTLSVerifyPolicy()

		if err := c.OktetoContextWriter.Write(); err != nil {
			return err
		}
	}

	if created && ctxOptions.IsOkteto {
		oktetoLog.Success("Context '%s' created", okteto.RemoveSchema(ctxOptions.Context))
	}

	if ctxOptions.IsCtxCommand {
		oktetoLog.Success("Using %s @ %s", okteto.GetContext().Namespace, okteto.RemoveSchema(ctxStore.CurrentContext))
		if oktetoLog.GetOutputFormat() == oktetoLog.JSONFormat {
			if err := showCurrentCtxJSON(); err != nil {
				return err
			}
		}
	}

	return nil
}

// getClusterMetadata runs the user query GetClusterMetadata and returns the response
func getClusterMetadata(ctx context.Context, namespace string, okClientProvider oktetoClientProvider) (types.ClusterMetadata, error) {
	okClient, err := okClientProvider.Provide()
	if err != nil {
		return types.ClusterMetadata{}, err
	}
	return okClient.User().GetClusterMetadata(ctx, namespace)
}

func hasAccessToNamespace(ctx context.Context, c *Command, ctxOptions *Options) (bool, error) {
	if ctxOptions.IsOkteto {
		okClient, err := c.OktetoClientProvider.Provide()
		if err != nil {
			return false, err
		}

		hasOktetoClientAccess, err := utils.HasAccessToOktetoClusterNamespace(ctx, ctxOptions.Namespace, okClient)
		if err != nil {
			return false, err
		}

		return hasOktetoClientAccess, nil
	} else {
		k8sClient, _, err := c.K8sClientProvider.Provide(okteto.GetContext().Cfg)
		if err != nil {
			return false, err
		}

		hasK8sClientAccess, err := utils.HasAccessToK8sClusterNamespace(ctx, ctxOptions.Namespace, k8sClient)
		if err != nil {
			return false, err
		}

		return hasK8sClientAccess, nil
	}
}

func (c *Command) initOktetoContext(ctx context.Context, ctxOptions *Options) error {
	var userContext *types.UserContext
	userContext, err := getLoggedUserContext(ctx, c, ctxOptions)
	if err != nil {
		// if an expired token is explicitly used, an error informing of the situation
		// should be returned instead of automatically generating a new token
		if !ctxOptions.InferredToken && errors.Is(err, oktetoErrors.ErrTokenExpired) {
			return oktetoErrors.UserError{
				E:    err,
				Hint: "A new token is required. More information on how to generate one here: https://www.okteto.com/docs/core/credentials/personal-access-tokens/",
			}
		}
		if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.GetContext().Name).Error() && ctxOptions.IsCtxCommand {
			oktetoLog.Warning("Your token is invalid. Generating a new one...")
			ctxOptions.Token = ""
			userContext, err = getLoggedUserContext(ctx, c, ctxOptions)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if ctxOptions.Namespace == "" {
		ctxOptions.Namespace = userContext.User.Namespace
	}

	clusterMetadata, err := getClusterMetadata(ctx, ctxOptions.Namespace, c.OktetoClientProvider)
	if err != nil {
		oktetoLog.Infof("error getting cluster metadata: %v", err)
		return err
	}

	// once we have namespace and user identify we are able to retrieve the dynamic token for the namespace
	err = c.kubetokenController.updateOktetoContextToken(userContext)
	if err != nil {
		// TODO: when the static token feature gets removed, we must return an error to the user here
		oktetoLog.Infof("error updating okteto context token: %v", err)
	}

	okteto.AddOktetoContext(ctxOptions.Context, &userContext.User, ctxOptions.Namespace, userContext.User.Namespace)
	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		cfg = kubeconfig.Create()
	}
	if err := okteto.AddOktetoCredentialsToCfg(cfg, &userContext.Credentials, ctxOptions.Namespace, userContext.User.ID, *okteto.GetContext()); err != nil {
		return err
	}

	okteto.GetContext().Cfg = cfg
	okteto.GetContext().IsOkteto = true
	okteto.GetContext().IsInsecure = okteto.IsInsecureSkipTLSVerifyPolicy()

	okteto.GetContext().IsTrial = clusterMetadata.IsTrialLicense
	okteto.GetContext().CompanyName = clusterMetadata.CompanyName

	exportPlatformVariablesToEnv(userContext.PlatformVariables)

	os.Setenv(model.OktetoUserNameEnvVar, okteto.GetContext().Username)

	return nil
}

func getLoggedUserContext(ctx context.Context, c *Command, ctxOptions *Options) (*types.UserContext, error) {
	user, err := c.LoginController.AuthenticateToOktetoCluster(ctx, ctxOptions.Context, ctxOptions.Token)
	if err != nil {
		return nil, err
	}

	ctxOptions.Token = user.Token

	okteto.GetContext().Token = user.Token
	okteto.SetInsecureSkipTLSVerifyPolicy(okteto.GetContext().IsStoredAsInsecure)

	if ctxOptions.Namespace == "" {
		ctxOptions.Namespace = user.Namespace
	}

	userContext, err := c.getUserContext(ctx, okteto.GetContext().Name, okteto.GetContext().Namespace, okteto.GetContext().Token)
	if err != nil {
		return nil, err
	}

	return userContext, nil
}

func (*Command) initKubernetesContext(ctxOptions *Options) error {
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

	kubeCtx.Namespace = okteto.GetContext().Namespace
	cfg.CurrentContext = okteto.GetContext().Name
	okteto.GetContext().Cfg = cfg
	okteto.GetContext().IsOkteto = false

	return nil
}

func (c Command) getUserContext(ctx context.Context, ctxName, ns, token string) (*types.UserContext, error) {
	client, err := c.OktetoClientProvider.Provide(
		okteto.WithCtxName(ctxName),
		okteto.WithToken(token),
	)
	if err != nil {
		return nil, err
	}

	retries := 0
	for retries <= 3 {
		userContext, err := client.User().GetContext(ctx, ns)

		if err != nil {
			if errors.Is(err, oktetoErrors.ErrTokenExpired) {
				return nil, err
			}

			if oktetoErrors.IsForbidden(err) {
				if err := c.OktetoContextWriter.Write(); err != nil {
					oktetoLog.Infof("error updating okteto contexts: %v", err)
					return nil, fmt.Errorf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
				}
				return nil, oktetoErrors.NotLoggedError{
					Context: okteto.GetContext().Name,
				}
			}

			// If there is a TLS error, don't continue the loop and return the raw error
			if oktetoErrors.IsX509(err) {
				return nil, err
			}

			if errors.Is(err, oktetoErrors.ErrInvalidLicense) {
				return nil, err
			}

			if oktetoErrors.IsNotFound(err) {
				// fallback to personal namespace using empty string as param
				userContext, err = client.User().GetContext(ctx, "")
				if err != nil {
					return nil, err
				}
			}
		}

		if err != nil {
			oktetoLog.Info(err)
			retries++
			continue
		}

		// If userID is not on context config file we add it and save it.
		// this prevents from relogin to actual users
		if okteto.GetContext().UserID == "" && okteto.GetContext().IsOkteto {
			okteto.GetContext().UserID = userContext.User.ID
			if err := c.OktetoContextWriter.Write(); err != nil {
				oktetoLog.Infof("error updating okteto contexts: %v", err)
				return nil, fmt.Errorf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextsStorePath())
			}
		}

		return userContext, nil
	}
	return nil, oktetoErrors.ErrInternalServerError
}

func (*Command) loadDotEnv(fs afero.Fs, setEnvFunc func(key, value string) error) error {
	dotEnvFile := ".env"
	if filesystem.FileExistsWithFilesystem(dotEnvFile, fs) {
		content, err := afero.ReadFile(fs, dotEnvFile)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}
		expanded, err := env.ExpandEnv(string(content))
		if err != nil {
			return fmt.Errorf("error expanding dot env file: %w", err)
		}
		vars, err := godotenv.UnmarshalBytes([]byte(expanded))
		if err != nil {
			return fmt.Errorf("error parsing dot env file: %w", err)
		}
		for k, v := range vars {
			err := setEnvFunc(k, v)
			if err != nil {
				return fmt.Errorf("error setting env var: %w", err)
			}
			oktetoLog.AddMaskedWord(v)
		}
	}
	return nil
}

func isUrl(u string) bool {
	parsedUrl, err := url.Parse(u)
	if err != nil {
		oktetoLog.Infof("could not parse %s", u)
		return false
	}
	return parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

func showCurrentCtxJSON() error {
	okCtx := okteto.GetContext().ToViewer()
	ctxRaw, err := json.MarshalIndent(okCtx, "", "\t")
	if err != nil {
		return err
	}
	fmt.Println(string(ctxRaw))
	return nil
}
