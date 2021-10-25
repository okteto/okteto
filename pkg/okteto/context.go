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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
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
	Name            string               `json:"name"`
	UserID          string               `json:"-"`
	Username        string               `json:"-"`
	Token           string               `json:"token,omitempty"`
	Namespace       string               `json:"namespace"`
	Cfg             *clientcmdapi.Config `json:"-"`
	Buildkit        string               `json:"buildkit,omitempty"`
	Registry        string               `json:"-"`
	Certificate     string               `json:"certificate,omitempty"`
	GlobalNamespace string               `json:"-"`
	Analytics       bool                 `json:"-"`
	ClusterType     string               `json:"-"`
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
		log.Infof("error accessing deprecated okteto token '%s': %v", config.GetTokenPathDeprecated(), err)
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
		log.Infof("error reading okteto certificate: %v", err)
		return
	}

	ctxStore.Contexts[token.URL] = &OktetoContext{
		Name:        token.URL,
		Namespace:   kubeconfig.CurrentNamespace(config.GetKubeconfigPath()),
		Token:       token.Token,
		Buildkit:    token.Buildkit,
		Certificate: base64.StdEncoding.EncodeToString(certificateBytes),
	}
	ctxStore.CurrentContext = token.URL

	if err := WriteOktetoContextConfig(); err != nil {
		log.Infof("error writing okteto context: %v", err)
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

// K8sContextToOktetoUrl translates k8s contexts like cloud_okteto_com to hettps://cloud.okteto.com
func K8sContextToOktetoUrl(k8sContext, k8sNamespace string) string {
	if IsOktetoURL(k8sContext) {
		return k8sContext
	}

	ctxStore := ContextStore()
	//check if belongs to the okteto contexts
	for name := range ctxStore.Contexts {
		if IsOktetoURL(name) && UrlToKubernetesContext(name) == k8sContext {
			return name
		}
	}

	cfg := kubeconfig.Get(config.GetKubeconfigPath())
	if cfg == nil {
		return k8sContext
	}

	cfg.CurrentContext = k8sContext
	c, _, err := getK8sClient(cfg)
	if err != nil {
		log.Infof("error getting k8s client: %v", err)
		return k8sContext
	}

	ctx := context.Background()
	//check the namespace label
	if k8sNamespace == "" {
		k8sNamespace = cfg.Contexts[k8sContext].Namespace
	}

	if k8sNamespace == "" {
		return k8sContext
	}

	n, err := c.CoreV1().Namespaces().Get(ctx, k8sNamespace, metav1.GetOptions{})
	if err != nil {
		log.Debugf("error accessing current namespace: %v", err)
		return k8sContext
	}
	if _, ok := n.Labels[model.DevLabel]; ok {
		return n.Annotations[model.OktetoURLAnnotation]
	}

	return k8sContext
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
			log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
		}

		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields() // Force errors

		ctxStore := &OktetoContextStore{}
		if err := dec.Decode(&ctxStore); err != nil {
			log.Errorf("error decoding okteto contexts: %v", err)
			log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
		}
		CurrentStore = ctxStore

		if _, ok := CurrentStore.Contexts[CurrentStore.CurrentContext]; !ok {
			log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
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
		log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	octx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		log.Info("ContextStore().CurrentContext not in ContextStore().Contexts")
		log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
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
	name = strings.TrimSuffix(name, "/")
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

func AddOktetoCredentialsToCfg(cfg *clientcmdapi.Config, cred *Credential, namespace, userName, oktetoURL string) {
	// If the context is being initialized within the execution of `okteto deploy` deploy command it should not
	// write the Okteto credentials into the kubeconfig. It would overwrite the proxy settings
	if os.Getenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT") == "true" {
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
	c, config, err := getK8sClient(Context().Cfg)
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

func RemoveSchema(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	return strings.TrimPrefix(u.String(), fmt.Sprintf("%s://", u.Scheme))
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
