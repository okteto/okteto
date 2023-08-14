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
	"sync"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	tokenRequestKind = "TokenRequest"
)

var (
	errEmptyNamespace           = errors.New("namespace cannot be empty")
	errEmptyContext             = errors.New("context name cannot be empty")
	errTokenRequestNotSupported = errors.New("kubernetes cluster does not support TokenRequest")

	valdationTimeout = 3 * time.Second
)

// validator is the interface that wraps the Validate method.
type validator interface {
	Validate(ctx context.Context) error
}

type preReqCfg struct {
	ctxName string
	ns      string

	k8sClientProvider    okteto.K8sClientProvider
	oktetoClientProvider types.OktetoClientProvider
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

func withK8sClientProvider(k8sClientProvider okteto.K8sClientProvider) option {
	return func(cfg *preReqCfg) {
		cfg.k8sClientProvider = k8sClientProvider
	}
}

func withOktetoClientProvider(oktetoClientProvider types.OktetoClientProvider) option {
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

	k8sClientProvider    okteto.K8sClientProvider
	oktetoClientProvider types.OktetoClientProvider
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
	oktetoLog.Info("validating pre-reqs")

	ctx, cancel := context.WithTimeout(ctx, valdationTimeout)
	defer cancel()

	err := newCtxValidator(v.ctxName, v.k8sClientProvider).Validate(ctx)
	if err != nil {
		return fmt.Errorf("invalid context: %w", err)
	}

	var wg sync.WaitGroup
	validators := []validator{
		newNsValidator(v.ns, v.oktetoClientProvider),
		newOktetoSupportValidator(ctx, v.ctxName, v.ns, v.k8sClientProvider, v.oktetoClientProvider),
	}
	errChan := make(chan error, len(validators))
	for _, preReqValidator := range validators {
		wg.Add(1)
		go func(v validator) {
			defer wg.Done()
			err := v.Validate(ctx)
			if err != nil {
				cancel()
			}
			errChan <- err
		}(preReqValidator)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}

// ctxValidator checks that the ctx use to execute the command is an okteto context
// that has already being added to your okteto context
type ctxValidator struct {
	ctxName           string
	k8sClientProvider okteto.K8sClientProvider
}

func newCtxValidator(ctxName string, k8sClientProvider okteto.K8sClientProvider) *ctxValidator {
	return &ctxValidator{
		ctxName:           ctxName,
		k8sClientProvider: k8sClientProvider,
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
	result := make(chan error, 1)
	go func() {
		if v.ctxName == "" {
			result <- errEmptyContext
			return
		}
		if !isURL(v.ctxName) {
			v.ctxName = okteto.K8sContextToOktetoUrl(ctx, v.ctxName, "", v.k8sClientProvider)
		}
		okCtx, exists := okteto.ContextStore().Contexts[v.ctxName]
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

// nsValidator validates that the namespace is valid and accessible
type nsValidator struct {
	ns                   string
	nsAccessChecker      nsAccessChecker
	previewAccessChecker previewAccessChecker
}

type errNamespaceForbidden struct {
	ns string
}

func (e errNamespaceForbidden) Error() string {
	return fmt.Sprintf("you don't have access to the namespace '%s'", e.ns)
}

type errPreviewForbidden struct {
	ns string
}

func (e errPreviewForbidden) Error() string {
	return fmt.Sprintf("you don't have access to the preview '%s'", e.ns)
}

type nsAccessChecker struct {
	oktetoClientProvider types.OktetoClientProvider
}

func newOktetoNsAccessChecker(oktetoClientProvider types.OktetoClientProvider) nsAccessChecker {
	return nsAccessChecker{
		oktetoClientProvider: oktetoClientProvider,
	}
}

func (c *nsAccessChecker) hasAccess(ctx context.Context, ns string) (bool, error) {
	okClient, err := c.oktetoClientProvider.Provide()
	if err != nil {
		return false, err
	}
	nList, err := okClient.Namespaces().List(ctx)
	if err != nil {
		return false, err
	}

	for i := range nList {
		if nList[i].ID == ns {
			return true, nil
		}
	}
	return false, errNamespaceForbidden{ns}
}

type previewAccessChecker struct {
	oktetoClientProvider types.OktetoClientProvider
}

func newOktetoPreviewAccessChecker(oktetoClientProvider types.OktetoClientProvider) previewAccessChecker {
	return previewAccessChecker{
		oktetoClientProvider: oktetoClientProvider,
	}
}

func (c *previewAccessChecker) hasAccess(ctx context.Context, preview string) (bool, error) {
	okClient, err := c.oktetoClientProvider.Provide()
	if err != nil {
		return false, err
	}
	previewList, err := okClient.Previews().List(ctx, []string{})
	if err != nil {
		return false, err
	}

	for i := range previewList {
		if previewList[i].ID == preview {
			return true, nil
		}
	}
	return false, errPreviewForbidden{preview}
}

// newNsValidator returns a new nsValidator instance
func newNsValidator(ns string, oktetoClientProvider types.OktetoClientProvider) *nsValidator {
	return &nsValidator{
		ns:                   ns,
		nsAccessChecker:      newOktetoNsAccessChecker(oktetoClientProvider),
		previewAccessChecker: newOktetoPreviewAccessChecker(oktetoClientProvider),
	}
}

func (v *nsValidator) Validate(ctx context.Context) error {
	result := make(chan error, 1)
	go func() {
		if v.ns == "" {
			result <- errEmptyNamespace
			return
		}

		hasAccess, err := v.nsAccessChecker.hasAccess(ctx, v.ns)
		if err != nil && !errors.Is(err, errNamespaceForbidden{v.ns}) {
			result <- err
			return
		}
		if hasAccess {
			result <- nil
			return
		}
		hasAccess, err = v.previewAccessChecker.hasAccess(ctx, v.ns)
		if err != nil && !errors.Is(err, errPreviewForbidden{v.ns}) {
			result <- err
			return
		}
		if hasAccess {
			result <- nil
			return
		}
		result <- errNamespaceForbidden{v.ns}
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type k8sTokenRequestSupportValidator struct {
	k8sClientProvider okteto.K8sClientProvider
	getApiConfig      func() *clientcmdapi.Config
}

func newK8sSupportValidator(k8sClientProvider okteto.K8sClientProvider) *k8sTokenRequestSupportValidator {
	return &k8sTokenRequestSupportValidator{
		k8sClientProvider: k8sClientProvider,
		getApiConfig: func() *clientcmdapi.Config {
			return okteto.Context().Cfg
		},
	}
}

func (v *k8sTokenRequestSupportValidator) Validate(ctx context.Context) error {
	result := make(chan error, 1)
	go func() {
		c, _, err := v.k8sClientProvider.Provide(v.getApiConfig())
		if err != nil {
			result <- fmt.Errorf("error creating kubernetes client: %w", err)
			return
		}

		authGroupVersion := schema.GroupVersion{Group: authenticationv1.GroupName, Version: "v1"}
		apiResourceList, err := c.Discovery().ServerResourcesForGroupVersion(authGroupVersion.String())
		if err != nil {
			result <- errTokenRequestNotSupported
			return
		}

		for _, apiResource := range apiResourceList.APIResources {
			if apiResource.Kind == tokenRequestKind {
				result <- nil
				return
			}
		}
		result <- errTokenRequestNotSupported
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
	oktetoClientProvider types.OktetoClientProvider
}

func newOktetoSupportValidator(ctx context.Context, ctxName, ns string, k8sClientProvider okteto.K8sClientProvider, oktetoClientProvider types.OktetoClientProvider) *oktetoSupportValidator {
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
