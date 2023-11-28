package build

import (
	"crypto/x509"
	"fmt"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OktetoContextInterface interface {
	GetCurrentName() string
	GetCurrentCfg() *clientcmdapi.Config
	GetCurrentNamespace() string
	GetGlobalNamespace() string
	GetCurrentBuilder() string
	GetCurrentCertStr() string
	GetCurrentCert() *x509.Certificate
	GetCurrentToken() string
	GetCurrentUser() string
	GetCurrentRegister() string
	ExistsContext() bool
	IsOkteto() bool
	IsInsecure() bool
	UseContextByBuilder()
}

type OktetoContext struct {
	Store *okteto.OktetoContextStore
}

func (oc *OktetoContext) UseContextByBuilder() {
	currentBuilder := oc.GetCurrentBuilder()
	for _, octx := range oc.Store.Contexts {
		if octx.IsOkteto && octx.Builder == currentBuilder {
			oc.getCurrentOktetoContext().Token = octx.Token
			oc.getCurrentOktetoContext().Certificate = octx.Certificate
		}
	}
}

func (oc *OktetoContext) GetCurrentBuilder() string {
	return oc.getCurrentOktetoContext().Builder
}

func (oc *OktetoContext) GetCurrentName() string {
	return oc.getCurrentOktetoContext().Name
}

func (oc *OktetoContext) GetCurrentCertStr() string {
	return oc.getCurrentOktetoContext().Certificate
}

func (oc *OktetoContext) GetCurrentCert() *x509.Certificate {
	return nil
}

func (oc *OktetoContext) GetCurrentToken() string {
	return oc.getCurrentOktetoContext().Token
}

func (oc *OktetoContext) GetCurrentUser() string {
	return oc.getCurrentOktetoContext().UserID
}

func (oc *OktetoContext) GetCurrentRegister() string {
	return oc.getCurrentOktetoContext().Registry
}

func (oc *OktetoContext) IsOkteto() bool {
	return oc.getCurrentOktetoContext().IsOkteto
}

func (oc *OktetoContext) ExistsContext() bool {
	return oc.getCurrentOktetoContext() != nil
}

func (oc *OktetoContext) IsInsecure() bool {
	return oc.getCurrentOktetoContext().IsInsecure
}

func (oc *OktetoContext) GetCurrentCfg() *clientcmdapi.Config {
	return oc.getCurrentOktetoContext().Cfg
}

func (oc *OktetoContext) GetCurrentNamespace() string {
	return oc.getCurrentOktetoContext().Namespace
}

func (oc *OktetoContext) GetGlobalNamespace() string {
	return oc.getCurrentOktetoContext().GlobalNamespace
}

func (oc *OktetoContext) GetTokenByContextName(name string) (string, error) {
	ctx, ok := oc.Store.Contexts[name]
	if !ok {
		return "", fmt.Errorf("context '%s' not found. ", name)
	}

	return ctx.Token, nil
}

func (oc *OktetoContext) getCurrentOktetoContext() *okteto.OktetoContext {
	if oc.Store.CurrentContext == "" {
		oktetoLog.Info("ContextStore().CurrentContext is empty")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	octx, ok := oc.Store.Contexts[oc.Store.CurrentContext]
	if !ok {
		oktetoLog.Info("ContextStore().CurrentContext not in ContextStore().Contexts")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	return octx
}
