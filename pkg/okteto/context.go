// Copyright 2022 The Okteto Authors
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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OktetoContextStore struct {
	Contexts       map[string]*OktetoContext `json:"contexts"`
	CurrentContext string                    `json:"current-context"`
}

const (
	localClusterType  = "local"
	remoteClusterType = "remote"
)

var (
	CurrentStore *OktetoContextStore
)

// OktetoContext contains the information related to an okteto context
type OktetoContext struct {
	Name              string               `json:"name" yaml:"name,omitempty"`
	UserID            string               `json:"id,omitempty" yaml:"id,omitempty"`
	Username          string               `json:"username,omitempty" yaml:"username,omitempty"`
	Token             string               `json:"token,omitempty" yaml:"token,omitempty"`
	Namespace         string               `json:"namespace" yaml:"namespace,omitempty"`
	Cfg               *clientcmdapi.Config `json:"-"`
	Builder           string               `json:"builder,omitempty" yaml:"builder,omitempty"`
	Registry          string               `json:"registry,omitempty" yaml:"registry,omitempty"`
	Certificate       string               `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	PersonalNamespace string               `json:"personalNamespace,omitempty" yaml:"personalNamespace,omitempty"`
	GlobalNamespace   string               `json:"-"`
	Analytics         bool                 `json:"-"`
	ClusterType       string               `json:"-"`
	IsOkteto          bool                 `json:"isOkteto"`
}

// InitContextWithDeprecatedToken initializes the okteto context if an old fashion exists and it matches the current kubernetes context
// this function is to make "okteto context" transparent to current Okteto Enterprise users, but it can be removed when people upgrade
func InitContextWithDeprecatedToken() {
	if !model.FileExists(config.GetTokenPathDeprecated()) {
		return
	}

	defer os.RemoveAll(config.GetTokenPathDeprecated())
	token, err := getTokenFromOktetoHome()
	if err != nil {
		oktetoLog.Infof("error accessing deprecated okteto token '%s': %v", config.GetTokenPathDeprecated(), err)
		return
	}

	k8sContext := UrlToKubernetesContext(token.URL)
	if kubeconfig.CurrentContext(config.GetKubeconfigPath()) != k8sContext {
		return
	}

	ctxStore := ContextStore()
	if _, ok := ctxStore.Contexts[token.URL]; ok {
		return
	}

	certificateBytes, err := os.ReadFile(config.GetCertificatePath())
	if err != nil {
		oktetoLog.Infof("error reading okteto certificate: %v", err)
		return
	}

	ctxStore.Contexts[token.URL] = &OktetoContext{
		Name:        token.URL,
		Namespace:   kubeconfig.CurrentNamespace(config.GetKubeconfigPath()),
		Token:       token.Token,
		Builder:     token.Buildkit,
		Certificate: base64.StdEncoding.EncodeToString(certificateBytes),
		IsOkteto:    true,
		UserID:      token.ID,
	}
	ctxStore.CurrentContext = token.URL

	if err := NewContextConfigWriter().Write(); err != nil {
		oktetoLog.Infof("error writing okteto context: %v", err)
	}
}

//ContextExists checks if an okteto context has been created
func ContextExists() bool {
	oktetoContextFolder := config.GetOktetoContextsStorePath()
	if _, err := os.Stat(oktetoContextFolder); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		oktetoLog.Fatalf("error accessing okteto context store '%s': %s", oktetoContextFolder, err)
	}
	return true
}

func UrlToKubernetesContext(uri string) string {
	u, _ := url.Parse(uri)
	return strings.ReplaceAll(u.Host, ".", "_")
}

// K8sContextToOktetoUrl translates k8s contexts like cloud_okteto_com to hettps://cloud.okteto.com
func K8sContextToOktetoUrl(ctx context.Context, k8sContext, k8sNamespace string, clientProvider K8sClientProvider) string {
	ctxStore := ContextStore()
	//check if belongs to the okteto contexts
	for name, oCtx := range ctxStore.Contexts {
		if oCtx.IsOkteto && UrlToKubernetesContext(name) == k8sContext {
			return name
		}
	}

	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		return k8sContext
	}

	cfg.CurrentContext = k8sContext
	c, _, err := clientProvider.Provide(cfg)
	if err != nil {
		oktetoLog.Infof("error getting k8s client: %v", err)
		return k8sContext
	}

	//check the namespace label
	if k8sNamespace == "" {
		k8sNamespace = cfg.Contexts[k8sContext].Namespace
	}

	if k8sNamespace == "" {
		return k8sContext
	}

	n, err := c.CoreV1().Namespaces().Get(ctx, k8sNamespace, metav1.GetOptions{})
	if err != nil {
		oktetoLog.Debugf("error accessing current namespace: %v", err)
		return k8sContext
	}
	if _, ok := n.Labels[model.DevLabel]; ok {
		return n.Annotations[model.OktetoURLAnnotation]
	}

	return k8sContext
}

func IsContextInitialized() bool {
	ctxStore := ContextStore()
	return ctxStore.CurrentContext != ""
}

func IsOkteto() bool {
	return Context().IsOkteto
}

func ContextStore() *OktetoContextStore {
	if CurrentStore != nil {
		return CurrentStore
	}

	if ContextExists() {
		b, err := os.ReadFile(config.GetOktetoContextsStorePath())
		if err != nil {
			oktetoLog.Errorf("error reading okteto contexts: %v", err)
			oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
		}

		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields() // Force errors

		ctxStore := &OktetoContextStore{}
		if err := dec.Decode(&ctxStore); err != nil {
			oktetoLog.Errorf("error decoding okteto contexts: %v", err)
			oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
		}
		CurrentStore = ctxStore

		if _, ok := CurrentStore.Contexts[CurrentStore.CurrentContext]; !ok {
			return CurrentStore
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
		oktetoLog.Info("ContextStore().CurrentContext is empty")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	octx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		oktetoLog.Info("ContextStore().CurrentContext not in ContextStore().Contexts")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}

	return octx
}

func HasBeenLogged(oktetoURL string) bool {
	octxStore := ContextStore()
	_, ok := octxStore.Contexts[oktetoURL]
	return ok
}

func AddOktetoContext(name string, u *types.User, namespace, personalNamespace string) {
	CurrentStore = ContextStore()
	name = strings.TrimSuffix(name, "/")
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:              name,
		UserID:            u.ID,
		Username:          u.ExternalID,
		Token:             u.Token,
		Namespace:         namespace,
		PersonalNamespace: personalNamespace,
		GlobalNamespace:   u.GlobalNamespace,
		Builder:           u.Buildkit,
		Registry:          u.Registry,
		Certificate:       u.Certificate,
		Analytics:         u.Analytics,
	}
	CurrentStore.CurrentContext = name
}

func AddKubernetesContext(name, namespace, buildkitURL string) {
	CurrentStore = ContextStore()
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:      name,
		Namespace: namespace,
		Builder:   buildkitURL,
	}
	CurrentStore.CurrentContext = name
}

type ContextConfigWriterInterface interface {
	Write() error
}

//ContextConfigWriter writes the information about the context config into stdout
type ContextConfigWriter struct{}

func NewContextConfigWriter() *ContextConfigWriter {
	return &ContextConfigWriter{}
}

func (*ContextConfigWriter) Write() error {
	marshalled, err := json.MarshalIndent(ContextStore(), "", "\t")
	if err != nil {
		oktetoLog.Infof("failed to marshal context: %s", err)
		return fmt.Errorf("failed to generate your context")
	}

	contextFolder := config.GetOktetoContextFolder()
	if err := os.MkdirAll(contextFolder, 0700); err != nil {
		oktetoLog.Fatalf("failed to create %s: %s", contextFolder, err)
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

func AddOktetoCredentialsToCfg(cfg *clientcmdapi.Config, cred *types.Credential, namespace, userName, oktetoURL string) {
	// If the context is being initialized within the execution of `okteto deploy` deploy command it should not
	// write the Okteto credentials into the kubeconfig. It would overwrite the proxy settings
	if os.Getenv(model.OktetoWithinDeployCommandContextEnvVar) == "true" {
		return
	}

	clusterName := UrlToKubernetesContext(oktetoURL)
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
	c, config, err := getK8sClientWithApiConfig(Context().Cfg)
	if err == nil {
		Context().SetClusterType(config.Host)
	}
	return c, config, err
}

// GetDynamicClient returns a kubernetes dynamic client for the current okteto context
func GetDynamicClient() (dynamic.Interface, *rest.Config, error) {
	if Context().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	return getDynamicClient(Context().Cfg)
}

// GetDiscoveryClient return a kubernetes discovery client for the current okteto context
func GetDiscoveryClient() (discovery.DiscoveryInterface, *rest.Config, error) {
	if Context().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	return getDiscoveryClient(Context().Cfg)
}

// GetSanitizedUsername returns the username of the authenticated user sanitized to be DNS compatible
func GetSanitizedUsername() string {
	octx := Context()
	return reg.ReplaceAllString(strings.ToLower(octx.Username), "-")
}

func (okctx *OktetoContext) ToUser() *types.User {
	u := &types.User{
		ID:              okctx.UserID,
		ExternalID:      okctx.Username,
		Token:           okctx.Token,
		Buildkit:        okctx.Builder,
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

func RemoveSchema(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	return strings.TrimPrefix(u.String(), fmt.Sprintf("%s://", u.Scheme))
}

func AddSchema(oCtx string) string {
	parsedUrl, err := url.Parse(oCtx)
	if err == nil {
		if parsedUrl.Scheme == "" {
			parsedUrl.Scheme = "https"
		}
		return parsedUrl.String()
	}
	return oCtx
}

func (okctx *OktetoContext) SetClusterType(clusterHost string) {
	if isLocalHostname(clusterHost) {
		okctx.ClusterType = localClusterType
	} else {
		okctx.ClusterType = remoteClusterType
	}
}

func isLocalHostname(clusterHost string) bool {
	u, err := url.Parse(clusterHost)
	host := ""
	if err == nil {
		host = u.Hostname()
		if host == "" {
			host = clusterHost
		}
	} else {
		host = clusterHost
	}

	ipAddress := net.ParseIP(host)
	return ipAddress.IsPrivate() || ipAddress.IsUnspecified() || ipAddress.IsLinkLocalUnicast() ||
		ipAddress.IsLoopback() || ipAddress.IsLinkLocalMulticast() || host == "kubernetes.docker.internal"
}
