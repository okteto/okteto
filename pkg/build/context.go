package build

import (
	"fmt"

	"github.com/okteto/okteto/pkg/okteto"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OktetoContextInterface interface {
	GetCurrentName() string
	GetCurrentCfg() *clientcmdapi.Config
	GetCurrentBuilder() string
	GetCurrentCert() string
	GetCurrentToken() string
	GetCurrentUser() string
	GetCurrentRegister() string
	IsOkteto() bool
	UseContextByBuilder()
}

type OktetoContext struct {
	Store *okteto.OktetoContextStore
}

func (oc *OktetoContext) UseContextByBuilder() {
	for _, octx := range oc.Store.Contexts {
		// if a context configures buildkit with an Okteto Cluster
		if octx.IsOkteto && octx.Builder == oc.GetCurrentBuilder() {
			okteto.Context().Token = octx.Token
			okteto.Context().Certificate = octx.Certificate
		}
	}
}

func (oc *OktetoContext) GetCurrentBuilder() string {
	return ""
}

func (oc *OktetoContext) GetCurrentName() string {
	return ""
}

func (oc *OktetoContext) GetCurrentCert() string {
	return ""
}

func (oc *OktetoContext) GetCurrentToken() string {
	return ""
}

func (oc *OktetoContext) GetCurrentUser() string {
	return ""
}

func (oc *OktetoContext) GetCurrentRegister() string {
	return ""
}

func (oc *OktetoContext) IsOkteto() bool {
	return false
}

func (oc *OktetoContext) GetCurrentCfg() *clientcmdapi.Config {
	return nil
}

func (oc *OktetoContext) GetTokenByContextName(name string) (string, error) {
	ctx, ok := oc.Store.Contexts[name]
	if !ok {
		return "", fmt.Errorf("context '%s' not found. ", name)
	}

	return ctx.Token, nil
}
