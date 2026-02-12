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
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ContextStore struct {
	Contexts       map[string]*Context `json:"contexts"`
	CurrentContext string              `json:"current-context"`
}

const (
	localClusterType  = "local"
	remoteClusterType = "remote"
	hoursInADay       = 24
	hoursInAWeek      = 168
)

var (
	CurrentStore *ContextStore
	reg          = regexp.MustCompile("[^A-Za-z0-9]+")
)

// Context contains the information related to an okteto context
type Context struct {
	Cfg                *clientcmdapi.Config `json:"-" yaml:"-"`
	Name               string               `json:"name" yaml:"name,omitempty"`
	UserID             string               `json:"id,omitempty" yaml:"id,omitempty"`
	Username           string               `json:"username,omitempty" yaml:"username,omitempty"`
	Token              string               `json:"token,omitempty" yaml:"token,omitempty"`
	Namespace          string               `json:"namespace" yaml:"namespace,omitempty"`
	Builder            string               `json:"builder,omitempty" yaml:"builder,omitempty"`
	Registry           string               `json:"registry,omitempty" yaml:"registry,omitempty"`
	Certificate        string               `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	PersonalNamespace  string               `json:"personalNamespace,omitempty" yaml:"personalNamespace,omitempty"`
	GlobalNamespace    string               `json:"-" yaml:"-"`
	ClusterType        string               `json:"-" yaml:"-"`
	CompanyName        string               `json:"-" yaml:"-"`
	IsOkteto           bool                 `json:"isOkteto,omitempty" yaml:"isOkteto,omitempty"`
	IsStoredAsInsecure bool                 `json:"isInsecure,omitempty" yaml:"isInsecure,omitempty"`
	IsInsecure         bool                 `json:"-" yaml:"-"`
	Analytics          bool                 `json:"-" yaml:"-"`
	IsTrial            bool                 `json:"-" yaml:"-"`
	DivertCRDSEnabled  bool                 `json:"-" yaml:"-"`
	Gateway            *GatewayMetadata     `json:"-" yaml:"-"`
}

// GatewayMetadata contains Gateway API configuration for the cluster
type GatewayMetadata struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// ContextViewer contains info to show
type ContextViewer struct {
	Name      string `json:"name" yaml:"name,omitempty"`
	Namespace string `json:"namespace" yaml:"namespace,omitempty"`
	Builder   string `json:"builder,omitempty" yaml:"builder,omitempty"`
	Registry  string `json:"registry,omitempty" yaml:"registry,omitempty"`
	Current   bool   `json:"current" yaml:"current"`
}

// InitContextWithDeprecatedToken initializes the okteto context if an old fashion exists and it matches the current kubernetes context
// this function is to make "okteto context" transparent to current Okteto Enterprise users, but it can be removed when people upgrade
func InitContextWithDeprecatedToken() {
	if !filesystem.FileExists(config.GetTokenPathDeprecated()) {
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

	ctxStore := GetContextStore()
	if _, ok := ctxStore.Contexts[token.URL]; ok {
		return
	}

	certificateBytes, err := os.ReadFile(config.GetCertificatePath())
	if err != nil {
		oktetoLog.Infof("error reading okteto certificate: %v", err)
		return
	}

	ctxStore.Contexts[token.URL] = &Context{
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

func getTokenFromOktetoHome() (*Token, error) {
	p := config.GetTokenPathDeprecated()

	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	currentToken := &Token{}
	if err := json.Unmarshal(b, currentToken); err != nil {
		return nil, err
	}

	return currentToken, nil
}

// ContextExists checks if an okteto context has been created
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
	u, err := url.Parse(uri)
	if err != nil {
		oktetoLog.Infof("error parsing url '%s': %v", uri, err)
		return uri
	}
	return strings.ReplaceAll(u.Host, ".", "_")
}

// K8sContextToOktetoUrl translates k8s contexts like okteto_example_com to https://okteto.example.com
func K8sContextToOktetoUrl(ctx context.Context, k8sContext, k8sNamespace string, clientProvider K8sClientProvider) string {
	ctxStore := GetContextStore()
	// check if belongs to the okteto contexts
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

	// check the namespace label
	if k8sNamespace == "" {
		cfgK8sNamespace, exists := cfg.Contexts[k8sContext]
		if exists {
			k8sNamespace = cfgK8sNamespace.Namespace
		} else {
			return k8sContext
		}
	}

	n, err := c.CoreV1().Namespaces().Get(ctx, k8sNamespace, metav1.GetOptions{})
	if err != nil {
		oktetoLog.Debugf("error accessing current namespace: %v", err)
		return k8sContext
	}
	if _, ok := n.Labels[constants.DevLabel]; ok {
		return n.Annotations[constants.OktetoURLAnnotation]
	}

	return k8sContext
}

func IsContextInitialized() bool {
	ctxStore := GetContextStore()
	return ctxStore.CurrentContext != ""
}

func IsOkteto() bool {
	return GetContext().IsOkteto
}

func GetContextStore() *ContextStore {
	if CurrentStore != nil {
		return CurrentStore
	}

	if ContextExists() {
		ctxStore := GetContextStoreFromStorePath()
		if ctxStore.Contexts == nil {
			ctxStore.Contexts = map[string]*Context{}
		}
		CurrentStore = ctxStore
		return ctxStore
	}

	CurrentStore = &ContextStore{
		Contexts: map[string]*Context{},
	}
	return CurrentStore
}

func GetContextStoreFromStorePath() *ContextStore {
	b, err := os.ReadFile(config.GetOktetoContextsStorePath())
	if err != nil {
		oktetoLog.Errorf("error reading okteto contexts: %v", err)
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // Force errors

	ctxStore := &ContextStore{}
	if err := dec.Decode(&ctxStore); err != nil {
		oktetoLog.Errorf("error decoding okteto contexts: %v", err)
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	return ctxStore
}

func GetContext() *Context {
	c := GetContextStore()
	if c.CurrentContext == "" {
		oktetoLog.Info("GetContextStore().CurrentContext is empty")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}
	octx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		oktetoLog.Info("GetContextStore().CurrentContext not in GetContextStore().Contexts")
		oktetoLog.Fatalf(oktetoErrors.ErrCorruptedOktetoContexts, config.GetOktetoContextFolder())
	}

	return octx
}

func HasBeenLogged(oktetoURL string) bool {
	octxStore := GetContextStore()
	_, ok := octxStore.Contexts[oktetoURL]
	return ok
}

func AddOktetoContext(name string, u *types.User, namespace, personalNamespace string) {
	CurrentStore = GetContextStore()
	name = strings.TrimSuffix(name, "/")
	if CurrentStore.Contexts[name] == nil {
		CurrentStore.Contexts[name] = &Context{}
	}
	current := CurrentStore.Contexts[name]
	current.Name = name
	current.UserID = u.ID
	current.Username = u.ExternalID
	current.Token = u.Token
	current.Namespace = namespace
	current.PersonalNamespace = personalNamespace
	current.GlobalNamespace = u.GlobalNamespace
	current.Builder = u.Buildkit
	current.Registry = u.Registry
	current.Certificate = u.Certificate
	current.Analytics = u.Analytics

	CurrentStore.CurrentContext = name
}

func AddKubernetesContext(name, namespace string) {
	CurrentStore = GetContextStore()
	CurrentStore.Contexts[name] = &Context{
		Name:      name,
		Namespace: namespace,
		Analytics: true,
	}
	CurrentStore.CurrentContext = name
}

type ContextConfigWriterInterface interface {
	Write() error
}

// ContextConfigWriter writes the information about the context config into stdout
type ContextConfigWriter struct{}

func NewContextConfigWriter() *ContextConfigWriter {
	return &ContextConfigWriter{}
}

func (*ContextConfigWriter) Write() error {
	marshalled, err := json.MarshalIndent(GetContextStore(), "", "\t")
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
			return fmt.Errorf("couldn't change context permissions: %w", err)
		}
	}

	if err := os.WriteFile(contextConfigPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save context: %w", err)
	}

	return nil
}

// AddOktetoCredentialsToCfg populates the provided kubernetes config using the provided credentials obtained from the Okteto API
func AddOktetoCredentialsToCfg(cfg *clientcmdapi.Config, cred *types.Credential, namespace, userName string, oktetoContext Context) error {
	// If the context is being initialized within the execution of `okteto deploy` deploy command it should not
	// write the Okteto credentials into the kubeconfig. It would overwrite the proxy settings
	if os.Getenv(constants.OktetoSkipConfigCredentialsUpdate) == "true" {
		return nil
	}

	clusterName := UrlToKubernetesContext(oktetoContext.Name)
	// create cluster
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}

	// If the certificate included in the credentials is empty, it means that the server is kubernetes.subdomain.
	// In that case, if the okteto context is insecure, we need to specify the certificate we have in our context.
	// If it is not insecure it means that is a public CA or a private one already installed in the machine
	cert := cred.Certificate
	if cred.Certificate == "" && oktetoContext.IsStoredAsInsecure {
		certPEM, err := base64.StdEncoding.DecodeString(oktetoContext.Certificate)
		if err != nil {
			oktetoLog.Debugf("couldn't decode context certificate from base64: %s", err)
			return fmt.Errorf("failed to decode context certificate: %w", err)
		}

		cert = string(certPEM)
	}

	cluster.CertificateAuthorityData = []byte(cert)
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
	context.Extensions[constants.OktetoExtension] = nil
	context.Cluster = clusterName
	context.AuthInfo = userName
	context.Namespace = namespace
	cfg.Contexts[clusterName] = context

	cfg.CurrentContext = clusterName

	return nil
}

// GetK8sClient returns a kubernetes client for the current okteto context and a kubernetes config object
func GetK8sClient() (*kubernetes.Clientset, *rest.Config, error) {
	if GetContext().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	c, config, err := getK8sClientWithApiConfig(GetContext().Cfg, nil)
	if err == nil {
		GetContext().SetClusterType(config.Host)
	}
	return c, config, err
}

// GetK8sClientWithLogger returns a kubernetes client for the current okteto context and a kubernetes config object
func GetK8sClientWithLogger(oktetoK8sLogger *io.K8sLogger) (*kubernetes.Clientset, *rest.Config, error) {
	if GetContext().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	c, config, err := getK8sClientWithApiConfig(GetContext().Cfg, oktetoK8sLogger)
	if err == nil {
		GetContext().SetClusterType(config.Host)
	}
	return c, config, err
}

// GetDynamicClient returns a kubernetes dynamic client for the current okteto context
func GetDynamicClient() (dynamic.Interface, *rest.Config, error) {
	if GetContext().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	return getDynamicClient(GetContext().Cfg)
}

// GetDiscoveryClient return a kubernetes discovery client for the current okteto context
func GetDiscoveryClient() (discovery.DiscoveryInterface, *rest.Config, error) {
	if GetContext().Cfg == nil {
		return nil, nil, fmt.Errorf("okteto context not initialized")
	}
	return getDiscoveryClient(GetContext().Cfg)
}

// GetSanitizedUsername returns the username of the authenticated user sanitized to be DNS compatible
func GetSanitizedUsername() string {
	octx := GetContext()
	return reg.ReplaceAllString(strings.ToLower(octx.Username), "-")
}

func (okctx *Context) ToUser() *types.User {
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

func (okctx *Context) SetClusterType(clusterHost string) {
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

// ToViewer transforms to a viewer struct
func (c *Context) ToViewer() *ContextViewer {
	builder := c.Builder
	if builder == "" {
		builder = "docker"
	}
	registry := c.Registry
	if builder == "" {
		registry = "-"
	}
	return &ContextViewer{
		Name:      c.Name,
		Namespace: c.Namespace,
		Builder:   builder,
		Registry:  registry,
		Current:   GetContext().Name == c.Name,
	}
}

// IsOktetoContext returns if the contextName param is Okteto
func IsOktetoContext(contextName string) bool {
	ctxStore := GetContextStore()
	selectedCtx, ok := ctxStore.Contexts[contextName]
	if !ok {
		return false
	}
	return selectedCtx.IsOkteto
}

func GetSubdomain() string {
	return strings.Replace(GetContext().Registry, "registry.", "", 1)
}

func GetContextCertificate() (*x509.Certificate, error) {
	if !ContextExists() {
		return nil, fmt.Errorf("okteto context not initialized")
	}
	certB64 := GetContext().Certificate
	certPEM, err := base64.StdEncoding.DecodeString(certB64)

	if err != nil {
		oktetoLog.Debugf("couldn't decode context certificate from base64: %s", err)
		return nil, err
	}

	block, _ := pem.Decode(certPEM)

	if block == nil {
		oktetoLog.Debugf("couldn't decode context certificate from pem: %s", err)
		return nil, fmt.Errorf("couldn't decode pem")
	}

	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		oktetoLog.Debugf("couldn't parse context certificate: %s", err)
		return nil, err
	}

	if _, err := cert.Verify(x509.VerifyOptions{}); err != nil { // skipcq: GO-S1031
		strictTLSOnce.Do(func() {
			oktetoLog.Debugf("certificate issuer %s", cert.Issuer)
			oktetoLog.Debugf("context certificate not trusted by system roots: %s", err)
			if !GetContext().IsInsecure {
				return
			}
			if cert.Issuer.CommonName == config.OktetoDefaultSelfSignedIssuer {
				hoursSinceInstall := time.Since(cert.NotBefore).Hours()
				switch {
				case hoursSinceInstall <= hoursInADay: // less than 1 day
					oktetoLog.Information("Your Okteto installation is using selfsigned certificates. Please switch to your own certificates before production use.")
				case hoursSinceInstall <= hoursInAWeek: // less than 1 week
					oktetoLog.Warning("Your Okteto installation has been using selfsigned certificates for more than a day. It's important to use your own certificates before production use.")
				default: // more than 1 week
					oktetoLog.Fail("[PLEASE READ] Your Okteto installation has been using selfsigned certificates for more than a week. It's important to use your own certificates before production use.")
				}
			}
		})
	}

	return cert, nil
}

func GetContextCertificateStateless(certB64 string) (*x509.Certificate, error) {
	certPEM, err := base64.StdEncoding.DecodeString(certB64)

	if err != nil {
		oktetoLog.Debugf("couldn't decode context certificate from base64: %s", err)
		return nil, err
	}

	block, _ := pem.Decode(certPEM)

	if block == nil {
		oktetoLog.Debugf("couldn't decode context certificate from pem: %s", err)
		return nil, fmt.Errorf("couldn't decode pem")
	}

	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		oktetoLog.Debugf("couldn't parse context certificate: %s", err)
		return nil, err
	}

	if _, err := cert.Verify(x509.VerifyOptions{}); err != nil { // skipcq: GO-S1031
		strictTLSOnce.Do(func() {
			oktetoLog.Debugf("certificate issuer %s", cert.Issuer)
			oktetoLog.Debugf("context certificate not trusted by system roots: %s", err)
			if !GetContext().IsInsecure {
				return
			}
			if cert.Issuer.CommonName == config.OktetoDefaultSelfSignedIssuer {
				hoursSinceInstall := time.Since(cert.NotBefore).Hours()
				hoursInADay := 24
				hoursInAWeek := 168
				switch {
				case hoursSinceInstall <= float64(hoursInADay): // less than 1 day
					oktetoLog.Information("Your Okteto installation is using selfsigned certificates. Please switch to your own certificates before production use.")
				case hoursSinceInstall <= float64(hoursInAWeek): // less than 1 week
					oktetoLog.Warning("Your Okteto installation has been using selfsigned certificates for more than a day. It's important to use your own certificates before production use.")
				default: // more than 1 week
					oktetoLog.Fail("[PLEASE READ] Your Okteto installation has been using selfsigned certificates for more than a week. It's important to use your own certificates before production use.")
				}
			}
		})
	}

	return cert, nil
}
