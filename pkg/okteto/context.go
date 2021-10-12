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

package okteto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OktetoContextStore struct {
	Contexts       map[string]*OktetoContext `json:"contexts"`
	CurrentContext string                    `json:"current-context"`
}

var CurrentStore *OktetoContextStore

// OktetoContext contains the information related to an okteto context
type OktetoContext struct {
	Name            string               `json:"name,omitempty" yaml:"name,omitempty"`
	UserID          string               `json:"-" yaml:"userID"`
	Username        string               `json:"-" yaml:"username"`
	Token           string               `json:"token,omitempty" yaml:"token"`
	Namespace       string               `json:"namespace,omitempty" yaml:"namespace"`
	Cfg             *clientcmdapi.Config `json:"-" yaml:"cfg""`
	Buildkit        string               `json:"buildkit" yaml:"buildkit"`
	Registry        string               `json:"-" yaml:"registry"`
	Certificate     string               `json:"certificate" yaml:"certificate"`
	GlobalNamespace string               `json:"-" yaml:"globalNamespace"`
	Analytics       bool                 `json:"-" yaml:"analytics"`
}

func AutomaticContextWithOktetoEnvVars(ctx context.Context, ctxResource *model.ContextResource) {
	if ctxResource.Token == "" {
		ctxResource.Token = os.Getenv("OKTETO_TOKEN")
	}

	if ctxResource.Token != "" {
		contextWithOktetoTokenEnvVar(ctxResource)
		return
	}

	if !model.FileExists(config.GetTokenPathDeprecated()) {
		return
	}
	defer os.RemoveAll(config.GetTokenPathDeprecated())
	token, err := getTokenFromOktetoHome()
	if err != nil {
		log.Infof("error accessing deprecated okteto token '%s': %v", config.GetTokenPathDeprecated(), err)
		return
	}

	contextWithDeprecatedToken(token, ctxResource)
}

func contextWithOktetoTokenEnvVar(ctxResource *model.ContextResource) {
	if ctxResource.Context == "" {
		ctxResource.Context = CloudURL
	}
	log.Infof("Using 'OKTETO_TOKEN' to access %s", ctxResource.Context)
	ctxStore := ContextStore()
	ctxStore.CurrentContext = ctxResource.Context

	if okCtx, ok := ctxStore.Contexts[ctxResource.Context]; ok {
		okCtx.Token = ctxResource.Token
		return
	}

	ctxStore.Contexts[ctxResource.Context] = &OktetoContext{
		Name:  ctxResource.Context,
		Token: ctxResource.Token,
	}
}

func contextWithDeprecatedToken(token *Token, ctxResource *model.ContextResource) {
	k8sContext := UrlToKubernetesContext(token.URL)
	if ctxResource.Context == k8sContext || ctxResource.Context == "" && kubeconfig.CurrentContext(config.GetKubeconfigPath()) == k8sContext {
		ctxStore := ContextStore()
		if _, ok := ctxStore.Contexts[token.URL]; ok {
			return
		}

		ctxStore.Contexts[token.URL] = &OktetoContext{
			Name:      token.URL,
			Namespace: kubeconfig.CurrentNamespace(config.GetKubeconfigPath()),
			Token:     token.Token,
		}
		ctxStore.CurrentContext = token.URL
		if err := WriteOktetoContextConfig(); err != nil {
			log.Infof("error writing okteto context: %v", err)
		}
	}

}

//ContextExists checks if an okteto context has been created
func ContextExists() bool {
	oktetoContextFolder := config.GetOktetoContextsStorePath()
	if _, err := os.Stat(oktetoContextFolder); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatalf("error accessing okteto context store '%s': %s", oktetoContextFolder, err)
	}
	return true
}

func UrlToKubernetesContext(uri string) string {
	u, _ := url.Parse(uri)
	return strings.ReplaceAll(u.Host, ".", "_")
}

func IsOktetoURL(name string) bool {
	u, err := url.Parse(name)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func IsOkteto() bool {
	return IsOktetoURL(Context().Name)
}

func ContextStore() *OktetoContextStore {
	if CurrentStore != nil {
		return CurrentStore
	}

	if ContextExists() {
		b, err := os.ReadFile(config.GetOktetoContextsStorePath())
		if err != nil {
			log.Errorf("error reading okteto contexts: %v", err)
			log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
		}

		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields() // Force errors

		ctxStore := &OktetoContextStore{}
		if err := dec.Decode(&ctxStore); err != nil {
			log.Errorf("error decoding okteto contexts: %v", err)
			log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
		}
		CurrentStore = ctxStore

		if _, ok := CurrentStore.Contexts[CurrentStore.CurrentContext]; !ok {
			log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
		}
		return CurrentStore
	}

	CurrentStore = &OktetoContextStore{
		Contexts: map[string]*OktetoContext{},
	}
	return CurrentStore
}

func Context() *OktetoContext {
	c := ContextStore()
	if c.CurrentContext == "" {
		log.Info("ContextStore().CurrentContext is empty")
		log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
	}
	octx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		log.Info("ContextStore().CurrentContext not in ContextStore().Contexts")
		log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
	}

	return octx
}

func HasBeenLogged(oktetoURL string) bool {
	octxStore := ContextStore()
	_, ok := octxStore.Contexts[oktetoURL]
	return ok
}

func AddOktetoContext(name string, u *User, namespace string) {
	CurrentStore = ContextStore()
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:            name,
		UserID:          u.ID,
		Username:        u.ExternalID,
		Token:           u.Token,
		Namespace:       namespace,
		GlobalNamespace: u.GlobalNamespace,
		Buildkit:        u.Buildkit,
		Registry:        u.Registry,
		Certificate:     u.Certificate,
		Analytics:       u.Analytics,
	}
	CurrentStore.CurrentContext = name
}

func AddKubernetesContext(name, namespace, buildkitURL string) {
	CurrentStore = ContextStore()
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:      name,
		Namespace: namespace,
		Buildkit:  buildkitURL,
	}
	CurrentStore.CurrentContext = name
}

func WriteOktetoContextConfig() error {
	marshalled, err := json.MarshalIndent(ContextStore(), "", "\t")
	if err != nil {
		log.Infof("failed to marshal context: %s", err)
		return fmt.Errorf("failed to generate your context")
	}

	contextFolder := config.GetOktetoContextFolder()
	if err := os.MkdirAll(contextFolder, 0700); err != nil {
		log.Fatalf("failed to create %s: %s", contextFolder, err)
	}

	contextConfigPath := config.GetOktetoContextsStorePath()
	if _, err := os.Stat(contextConfigPath); err == nil {
		err = os.Chmod(contextConfigPath, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change context permissions: %s", err)
		}
	}

	if err := os.WriteFile(contextConfigPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save context: %s", err)
	}

	return nil
}

//WriteKubeconfig updates the current context of a kubeconfig file
func WriteKubeconfig(cred *Credential, kubeConfigPath, namespace, userName, clusterName string) error {
	cfg := kubeconfig.Get(kubeConfigPath)
	if cfg == nil {
		cfg = kubeconfig.Create()
	}

	AddOktetoCredentialsToCfg(cfg, cred, namespace, userName, clusterName)

	return clientcmd.WriteToFile(*cfg, kubeConfigPath)
}

func AddOktetoCredentialsToCfg(cfg *clientcmdapi.Config, cred *Credential, namespace, userName, clusterName string) {
	// create cluster
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}

	cluster.CertificateAuthorityData = []byte(cred.Certificate)
	cluster.Server = cred.Server
	cfg.Clusters[clusterName] = cluster

	// create user
	user, ok := cfg.AuthInfos[userName]
	if !ok {
		user = clientcmdapi.NewAuthInfo()
	}
	user.Token = cred.Token
	cfg.AuthInfos[userName] = user

	// create context
	context, ok := cfg.Contexts[clusterName]
	if !ok {
		context = clientcmdapi.NewContext()
	}
	if context.Extensions == nil {
		context.Extensions = map[string]runtime.Object{}
	}
	context.Extensions[model.OktetoExtension] = nil
	context.Cluster = clusterName
	context.AuthInfo = userName
	context.Namespace = namespace
	cfg.Contexts[clusterName] = context

	cfg.CurrentContext = clusterName
}

func GetK8sClient() (*kubernetes.Clientset, *rest.Config, error) {
	if Context().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	return getK8sClient(Context().Cfg)
}

// GetSanitizedUsername returns the username of the authenticated user sanitized to be DNS compatible
func GetSanitizedUsername() string {
	octx := Context()
	return reg.ReplaceAllString(strings.ToLower(octx.Username), "-")
}

func (okctx *OktetoContext) ToUser() *User {
	u := &User{
		ID:              okctx.UserID,
		ExternalID:      okctx.Username,
		Token:           okctx.Token,
		Buildkit:        okctx.Buildkit,
		Registry:        okctx.Registry,
		Certificate:     okctx.Certificate,
		GlobalNamespace: okctx.GlobalNamespace,
		Analytics:       okctx.Analytics,
	}
	return u
}

func IsOktetoCloud() bool {
	octx := Context()
	switch octx.Name {
	case CloudURL, StagingURL:
		return true
	default:
		return false
	}
}
