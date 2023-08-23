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

package kubetoken

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	errEmptyContext = errors.New("context name cannot be empty")

	validationTimeout = 3000 * time.Second
)

type k8sClientProvider interface {
	Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error)
}

type preReqCfg struct {
	ctxName string
	ns      string

	k8sClientProvider    k8sClientProvider
	oktetoClientProvider oktetoClientProvider
	getContextStore      func() *okteto.OktetoContextStore
}

type option func(*preReqCfg)

func withCtxName(ctxName string) option {
	return func(cfg *preReqCfg) {
		cfg.ctxName = ctxName
	}
}

func withNamespace(ns string) option {
	return func(cfg *preReqCfg) {
		cfg.ns = ns
	}
}

func withK8sClientProvider(k8sClientProvider k8sClientProvider) option {
	return func(cfg *preReqCfg) {
		cfg.k8sClientProvider = k8sClientProvider
	}
}

func withOktetoClientProvider(oktetoClientProvider oktetoClientProvider) option {
	return func(cfg *preReqCfg) {
		cfg.oktetoClientProvider = oktetoClientProvider
	}
}

func defaultPreReqCfg() *preReqCfg {
	return &preReqCfg{
		k8sClientProvider:    okteto.NewK8sClientProvider(),
		oktetoClientProvider: okteto.NewOktetoClientProvider(),
		getContextStore:      okteto.ContextStore,
	}
}

// preReqValidator validates that all the pre-reqs to execute the command are met
type preReqValidator struct {
	ctxName string
	ns      string

	k8sClientProvider    k8sClientProvider
	oktetoClientProvider oktetoClientProvider
	getContextStore      getContextStoreFunc
	getCtxResource       initCtxOptsFunc
}

// newPreReqValidator returns a new preReqValidator
func newPreReqValidator(opts ...option) *preReqValidator {
	cfg := defaultPreReqCfg()
	for _, opt := range opts {
		opt(cfg)
	}
	return &preReqValidator{
		ctxName:              cfg.ctxName,
		ns:                   cfg.ns,
		k8sClientProvider:    cfg.k8sClientProvider,
		oktetoClientProvider: cfg.oktetoClientProvider,
		getContextStore:      cfg.getContextStore,
		getCtxResource:       getCtxResource,
	}
}

// Validate validates that all the pre-reqs to execute the command are met
func (v *preReqValidator) validate(ctx context.Context) error {
	oktetoLog.Info("validating pre-reqs for kubetoken")

	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()

	ctxResource := v.getCtxResource(v.ctxName, v.ns)

	err := newCtxValidator(ctxResource, v.k8sClientProvider, v.getContextStore).validate(ctx)
	if err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	err = newOktetoSupportValidator(ctx, ctxResource, v.k8sClientProvider, v.oktetoClientProvider).validate(ctx)
	if err != nil {
		return fmt.Errorf("invalid okteto support: %w", err)
	}
	return nil
}

func getCtxResource(ctxName, ns string) *contextCMD.ContextOptions {
	ctxResource := &contextCMD.ContextOptions{
		Context:   ctxName,
		Namespace: ns,
	}
	if ctxResource.Context == "" {
		ctxResource.InitFromContext()
		ctxResource.InitFromEnvVars()
	}
	return ctxResource
}

type getContextStoreFunc func() *okteto.OktetoContextStore
type initCtxOptsFunc func(string, string) *contextCMD.ContextOptions

// ctxValidator checks that the ctx use to execute the command is an okteto context
// that has already being added to your okteto context
type ctxValidator struct {
	ctxResource       *contextCMD.ContextOptions
	k8sClientProvider k8sClientProvider
	getContextStore   getContextStoreFunc
	k8sCtxToOktetoURL func(ctx context.Context, k8sContext string, k8sNamespace string, clientProvider okteto.K8sClientProvider) string
}

func newCtxValidator(ctxResource *contextCMD.ContextOptions, k8sClientProvider k8sClientProvider, getContextStore getContextStoreFunc) *ctxValidator {
	return &ctxValidator{
		ctxResource:       ctxResource,
		k8sClientProvider: k8sClientProvider,
		getContextStore:   getContextStore,
		k8sCtxToOktetoURL: okteto.K8sContextToOktetoUrl,
	}
}

type errOktetoContextNotFound struct {
	ctxName string
}

func (e errOktetoContextNotFound) Error() string {
	return fmt.Sprintf("context '%s' not found in the okteto context store", e.ctxName)
}

type errIsNotOktetoCtx struct {
	ctxName string
}

func (e errIsNotOktetoCtx) Error() string {
	return fmt.Sprintf("context '%s' is not an okteto context", e.ctxName)
}

func (v *ctxValidator) validate(ctx context.Context) error {
	oktetoLog.Debug("validating context for dynamic kubernetes token request")
	result := make(chan error, 1)
	go func() {
		if v.ctxResource.Context == "" {
			result <- errEmptyContext
			return
		}
		if !isURL(v.ctxResource.Context) {
			v.ctxResource.Context = v.k8sCtxToOktetoURL(ctx, v.ctxResource.Context, "", v.k8sClientProvider)
		}
		okCtx, exists := v.getContextStore().Contexts[v.ctxResource.Context]
		if !exists {
			if v.ctxResource.Token == "" {
				result <- errOktetoContextNotFound{v.ctxResource.Context}
				return
			}
		} else {
			if !okCtx.IsOkteto {
				result <- errIsNotOktetoCtx{v.ctxResource.Context}
				return
			}
		}
		result <- nil
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type oktetoSupportValidator struct {
	ctxResource          *contextCMD.ContextOptions
	oktetoClientProvider oktetoClientProvider
}

func newOktetoSupportValidator(ctx context.Context, ctxResource *contextCMD.ContextOptions, k8sClientProvider k8sClientProvider, oktetoClientProvider oktetoClientProvider) *oktetoSupportValidator {
	if !isURL(ctxResource.Context) {
		ctxResource.Context = okteto.K8sContextToOktetoUrl(ctx, ctxResource.Context, "", k8sClientProvider)
	}
	return &oktetoSupportValidator{
		ctxResource:          ctxResource,
		oktetoClientProvider: oktetoClientProvider,
	}
}

func (v *oktetoSupportValidator) validate(ctx context.Context) error {
	oktetoLog.Debug("validating okteto client support for kubetoken")
	result := make(chan error, 1)
	go func() {
		okClient, err := v.oktetoClientProvider.Provide(
			okteto.WithCtxName(v.ctxResource.Context),
			okteto.WithToken(v.ctxResource.Token),
		)
		if err != nil {
			result <- fmt.Errorf("error creating okteto client: %w", err)
			return
		}

		result <- okClient.Kubetoken().CheckService(v.ctxResource.Context, v.ctxResource.Namespace)
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isURL(ctxName string) bool {
	parsedUrl, err := url.Parse(ctxName)
	if err != nil {
		return false
	}
	return parsedUrl.Scheme != "" && parsedUrl.Host != ""
}
