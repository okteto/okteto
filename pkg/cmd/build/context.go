package build

import (
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OktetoContextInterface interface {
	GetCurrentName() string
	GetCurrentNamespace() string
	GetGlobalNamespace() string
	GetCurrentBuilder() string
	GetCurrentCertStr() string
	GetCurrentCfg() *clientcmdapi.Config
	GetCurrentToken() string
	GetCurrentUser() string
	GetCurrentRegister() string
	ExistsContext() bool
	IsOkteto() bool
	IsInsecure() bool
	UseContextByBuilder()
	GetTokenByContextName(name string) (string, error)
}
