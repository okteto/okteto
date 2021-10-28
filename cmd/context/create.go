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
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
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
		Short: "Create a new context",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			ctxStore := okteto.ContextStore()

			ctxOptions.Context = args[0]
			ctxOptions.initialCtx = ctxStore.CurrentContext
			ctxOptions.isOkteto = true

			err := Create(ctx, ctxStore, ctxOptions)
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

func Create(ctx context.Context, ctxStore *okteto.OktetoContextStore, ctxOptions *ContextOptions) error {
	var created bool
	var initialCurrentCtxNamespace string
	if _, ok := ctxStore.Contexts[ctxOptions.initialCtx]; ok {
		initialCurrentCtxNamespace = ctxStore.Contexts[ctxOptions.initialCtx].Namespace
	}

	ctxOptions.Context = strings.TrimSuffix(ctxOptions.Context, "/")
	if !ctxOptions.isOkteto {
		if !isValidCluster(ctxOptions.Context) {
			if okteto.IsOktetoURL(ctxOptions.Context) {
				return errors.UserError{
					E:    fmt.Errorf(errors.ErrInvalidContextOrOktetoCtx, ctxOptions.Context),
					Hint: fmt.Sprintf("Run 'okteto context create %s' to create a new okteto context or select one kubernetes context from:\n      %s", ctxOptions.Context, strings.Join(getKubernetesContextList(false), "\n      ")),
				}
			}
			return errors.UserError{
				E:    fmt.Errorf(errors.ErrInvalidContext, ctxOptions.Context),
				Hint: fmt.Sprintf("Valid Kubernetes contexts are:\n      %s", strings.Join(getKubernetesContextList(false), "\n      ")),
			}
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
	if created {
		log.Success("Context '%s' created", okteto.RemoveSchema(ctxOptions.Context))
	}

	if ctxOptions.initialCtx != ctxStore.CurrentContext || initialCurrentCtxNamespace != ctxStore.Contexts[ctxStore.CurrentContext].Namespace {
		log.Success("Switched to context '%s' @ %s", okteto.RemoveSchema(ctxStore.CurrentContext), ctxStore.Contexts[ctxStore.CurrentContext].Namespace)
	}
	return nil
}
