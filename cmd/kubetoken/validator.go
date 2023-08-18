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

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	errEmptyNamespace = errors.New("namespace cannot be empty")
	errEmptyContext   = errors.New("context name cannot be empty")

	valdationTimeout = 30000 * time.Second
)

type k8sClientProvider interface {
	Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error)
}

// validator is the interface that wraps the Validate method.
type validator interface {
	Validate(ctx context.Context) error
}

type preReqCfg struct {
	ctxName string
	ns      string

	k8sClientProvider    k8sClientProvider
	oktetoClientProvider oktetoClientProvider
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
	}
}

// preReqValidator validates that all the pre-reqs to execute the command are met
type preReqValidator struct {
	ctxName string
	ns      string

	k8sClientProvider    k8sClientProvider
	oktetoClientProvider oktetoClientProvider
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
	}
}

// Validate validates that all the pre-reqs to execute the command are met
func (v *preReqValidator) Validate(ctx context.Context) error {
	oktetoLog.Info("validating pre-reqs for kubetoken")

	ctx, cancel := context.WithTimeout(ctx, valdationTimeout)
	defer cancel()

	err := newCtxValidator(v.ctxName, v.k8sClientProvider).Validate(ctx)
	if err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	err = newOktetoSupportValidator(ctx, v.ctxName, v.ns, v.k8sClientProvider, v.oktetoClientProvider).Validate(ctx)
	if err != nil {
		return fmt.Errorf("invalid okteto support: %w", err)
	}
	return nil
}

// ctxValidator checks that the ctx use to execute the command is an okteto context
// that has already being added to your okteto context
type ctxValidator struct {
	ctxName           string
	k8sClientProvider k8sClientProvider
	getContextStore   func() *okteto.OktetoContextStore
	k8sCtxToOktetoURL func(ctx context.Context, k8sContext string, k8sNamespace string, clientProvider okteto.K8sClientProvider) string
}

func newCtxValidator(ctxName string, k8sClientProvider k8sClientProvider) *ctxValidator {
	return &ctxValidator{
		ctxName:           ctxName,
		k8sClientProvider: k8sClientProvider,
		getContextStore:   okteto.ContextStore,
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

func (v *ctxValidator) Validate(ctx context.Context) error {
	oktetoLog.Info("validating the context")
	result := make(chan error, 1)
	go func() {
		if v.ctxName == "" {
			result <- errEmptyContext
			return
		}
		if !isURL(v.ctxName) {
			v.ctxName = v.k8sCtxToOktetoURL(ctx, v.ctxName, "", v.k8sClientProvider)
		}
		okCtx, exists := v.getContextStore().Contexts[v.ctxName]
		if !exists {
			result <- errOktetoContextNotFound{v.ctxName}
			return
		}
		if !okCtx.IsOkteto {
			result <- errIsNotOktetoCtx{v.ctxName}
			return
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
	ctxName              string
	ns                   string
	oktetoClientProvider oktetoClientProvider
}

func newOktetoSupportValidator(ctx context.Context, ctxName, ns string, k8sClientProvider k8sClientProvider, oktetoClientProvider oktetoClientProvider) *oktetoSupportValidator {
	if !isURL(ctxName) {
		ctxName = okteto.K8sContextToOktetoUrl(ctx, ctxName, "", k8sClientProvider)
	}
	return &oktetoSupportValidator{
		ctxName:              ctxName,
		ns:                   ns,
		oktetoClientProvider: oktetoClientProvider,
	}
}

func (v *oktetoSupportValidator) Validate(ctx context.Context) error {
	oktetoLog.Info("validating okteto client support for kubetoken")
	result := make(chan error, 1)
	go func() {
		okClient, err := v.oktetoClientProvider.Provide()
		if err != nil {
			result <- fmt.Errorf("error creating okteto client: %w", err)
			return
		}

		result <- okClient.Kubetoken().CheckService(v.ctxName, v.ns)
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
