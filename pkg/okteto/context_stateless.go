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

package okteto

import (
	"fmt"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ContextInterface interface {
	GetCurrentName() string
	GetCurrentCfg() *clientcmdapi.Config
	GetNamespace() string
	GetGlobalNamespace() string
	GetCurrentBuilder() string
	GetCurrentCertStr() string
	GetCurrentToken() string
	GetCurrentUser() string
	ExistsContext() bool
	IsOktetoCluster() bool
	IsInsecure() bool
	UseContextByBuilder()
	GetTokenByContextName(name string) (string, error)
	GetRegistryURL() string
}

type ContextStateless struct {
	Store *ContextStore
}

func (oc *ContextStateless) UseContextByBuilder() {
	currentBuilder := oc.GetCurrentBuilder()
	for _, octx := range oc.Store.Contexts {
		if octx.IsOkteto && octx.Builder == currentBuilder {
			oc.getCurrentOktetoContext().Token = octx.Token
			oc.getCurrentOktetoContext().Certificate = octx.Certificate
		}
	}
}

func (oc *ContextStateless) GetCurrentBuilder() string {
	return oc.getCurrentOktetoContext().Builder
}

func (oc *ContextStateless) SetCurrentBuilder(builder string) {
	oc.getCurrentOktetoContext().Builder = builder
}

func (oc *ContextStateless) GetCurrentName() string {
	return oc.getCurrentOktetoContext().Name
}

func (oc *ContextStateless) SetCurrentName(name string) {
	oc.getCurrentOktetoContext().Name = name
}

func (oc *ContextStateless) GetCurrentCertStr() string {
	return oc.getCurrentOktetoContext().Certificate
}

func (oc *ContextStateless) SetCurrentCertStr(cert string) {
	oc.getCurrentOktetoContext().Certificate = cert
}

func (oc *ContextStateless) GetCurrentToken() string {
	return oc.getCurrentOktetoContext().Token
}

func (oc *ContextStateless) SetCurrentToken(token string) {
	oc.getCurrentOktetoContext().Token = token
}

func (oc *ContextStateless) GetCurrentUser() string {
	return oc.getCurrentOktetoContext().UserID
}

func (oc *ContextStateless) SetCurrentUser(user string) {
	oc.getCurrentOktetoContext().UserID = user
}

func (oc *ContextStateless) SetCurrentRegistry(reg string) {
	oc.getCurrentOktetoContext().Registry = reg
}

func (oc *ContextStateless) IsOktetoCluster() bool {
	return oc.getCurrentOktetoContext().IsOkteto
}

func (oc *ContextStateless) SetOktetoCluster(isOkteto bool) {
	oc.getCurrentOktetoContext().IsOkteto = isOkteto
}

func (oc *ContextStateless) ExistsContext() bool {
	return oc.getCurrentOktetoContext() != nil
}

func (oc *ContextStateless) IsInsecure() bool {
	return oc.getCurrentOktetoContext().IsInsecure
}

func (oc *ContextStateless) SetInsecure(insecure bool) {
	oc.getCurrentOktetoContext().IsInsecure = insecure
}

func (oc *ContextStateless) GetCurrentCfg() *clientcmdapi.Config {
	return oc.getCurrentOktetoContext().Cfg
}

func (oc *ContextStateless) SetCurrentCfg(cfg *clientcmdapi.Config) {
	oc.getCurrentOktetoContext().Cfg = cfg
}

func (oc *ContextStateless) GetNamespace() string {
	return oc.getCurrentOktetoContext().Namespace
}

func (oc *ContextStateless) SetNamespace(namespace string) {
	oc.getCurrentOktetoContext().Namespace = namespace
}

func (oc *ContextStateless) GetRegistryURL() string {
	return oc.getCurrentOktetoContext().Registry
}

func (oc *ContextStateless) GetGlobalNamespace() string {
	return oc.getCurrentOktetoContext().GlobalNamespace
}

func (oc *ContextStateless) SetGlobalNamespace(globalNamespace string) {
	oc.getCurrentOktetoContext().GlobalNamespace = globalNamespace
}

func (oc *ContextStateless) GetTokenByContextName(name string) (string, error) {
	ctx, ok := oc.Store.Contexts[name]
	if !ok {
		return "", fmt.Errorf("context '%s' not found. ", name)
	}

	return ctx.Token, nil
}

func (oc *ContextStateless) getCurrentOktetoContext() *Context {
	if oc.Store.CurrentContext == "" {
		oktetoLog.Info("GetContextStore().CurrentContext is empty")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	octx, ok := oc.Store.Contexts[oc.Store.CurrentContext]
	if !ok {
		oktetoLog.Info("GetContextStore().CurrentContext not in GetContextStore().Contexts")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	return octx
}
