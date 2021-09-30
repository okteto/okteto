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
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
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
	Name            string `json:"name,omitempty"`
	UserID          string `json:"userId,omitempty"`
	Username        string `json:"username,omitempty"`
	Token           string `json:"token,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	Kubeconfig      string `json:"kubeconfig,omitempty"`
	Buildkit        string `json:"buildkit,omitempty"`
	Registry        string `json:"registry,omitempty"`
	Certificate     string `json:"certificate,omitempty"`
	GlobalNamespace string `json:"globalNamespace,omitempty"`
}

func InitContextWithToken(ctx context.Context, oktetoUrl, oktetoToken string) error {
	kubeconfigFile := config.GetKubeconfigPath()
	oktetoClient, err := NewOktetoClientFromUrlAndToken(oktetoUrl, oktetoToken)
	if err != nil {
		return err
	}

	user, err := oktetoClient.queryUser(ctx)
	if err != nil {
		return err
	}

	cred, err := oktetoClient.GetCredentials(ctx)
	if err != nil {
		return err
	}

	if err := SetKubeContext(cred, kubeconfigFile, cred.Namespace, user.ID, UrlToContext(oktetoUrl)); err != nil {
		return fmt.Errorf("error updating kubernetes context: %v", err)
	}

	cfg := client.GetKubeconfig(kubeconfigFile)
	if err := SaveOktetoClusterContext(oktetoUrl, user, cred.Namespace, cfg); err != nil {
		return fmt.Errorf("error configuring okteto context: %v", err)
	}
	log.Information("Current context: %s\n    Run 'okteto context' if you need to change your context", UrlToContext(oktetoUrl))
	return nil
}

func InitContext(ctx context.Context, oktetoToken string) error {
	defer os.RemoveAll(config.GetTokenPathDeprecated())

	currentContext := os.Getenv("OKTETO_URL")
	if oktetoToken == "" {
		oktetoToken = os.Getenv("OKTETO_TOKEN")
	}
	if oktetoToken != "" {
		if currentContext == "" {
			currentContext = CloudURL
		}
		log.Information("Using 'OKTETO_TOKEN' to access %s", currentContext)
		return InitContextWithToken(ctx, currentContext, oktetoToken)
	}

	if contextExists() {
		return nil
	}

	if currentContext == "" {
		currentContext = os.Getenv(config.OktetoContextVariableName)
	}
	kubeconfigFile := config.GetKubeconfigPath()
	cfg := client.GetKubeconfig(kubeconfigFile)
	if currentContext == "" {
		currentContext = client.GetCurrentKubernetesContext(kubeconfigFile)
	}
	namespace := client.GetCurrentNamespace(kubeconfigFile)

	token, err := getTokenFromOktetoHome()
	if err == nil {
		return InitContextWithToken(ctx, token.URL, token.Token)
	}
	log.Infof("error accessing okteto token '%s': %v", config.GetTokenPathDeprecated(), err)

	if cfg == nil {
		return errors.ErrNoActiveOktetoContexts
	}

	if _, ok := cfg.Contexts[currentContext]; !ok {
		return fmt.Errorf("current kubernetes context '%s' doesn't exist in '%s'", currentContext, kubeconfigFile)
	}

	if err := SaveKubernetesClusterContext(currentContext, namespace, cfg, ""); err != nil {
		return fmt.Errorf("error configuring okteto context: %v", err)
	}
	log.Information("Current context: %s\n    Run 'okteto context' if you need to change your context", currentContext)

	return nil
}

//contextExists checks if an okteto context has been created
func contextExists() bool {
	oktetoContextFolder := config.GetOktetoContextFolder()
	if _, err := os.Stat(oktetoContextFolder); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatalf("error accessing okteto context store '%s': %s", oktetoContextFolder, err)
	}
	return true
}

func UrlToContext(uri string) string {
	u, _ := url.Parse(uri)
	return strings.ReplaceAll(u.Host, ".", "_")
}

func IsOktetoURL(name string) bool {
	u, err := url.Parse(name)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func IsOktetoContext() bool {
	octx := Context()
	return IsOktetoURL(octx.Name)
}

func ContextStore() *OktetoContextStore {
	if CurrentStore != nil {
		return CurrentStore
	}

	b, err := ioutil.ReadFile(config.GetOktetoContextsStorePath())
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
	return CurrentStore
}

func Context() *OktetoContext {
	c := ContextStore()
	if c.CurrentContext == "" {
		log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
	}
	octx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		log.Fatalf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
	}

	return octx
}

func SetCurrentContext(oktetoContext, namespace string) error {
	if oktetoContext == "" {
		oktetoContext = os.Getenv(config.OktetoContextVariableName)
	}

	if namespace == "" {
		namespace = os.Getenv("OKTETO_NAMESPACE")
	}

	octxStore := ContextStore()

	if oktetoContext != "" {
		if IsOktetoURL(oktetoContext) {
			if _, ok := octxStore.Contexts[oktetoContext]; !ok {
				//TODO: start login sequence
				return fmt.Errorf(errors.ErrOktetoContextNotFound, oktetoContext, oktetoContext)
			}
		} else {
			kubeconfigFile := config.GetKubeconfigPath()
			cfg := client.GetKubeconfig(kubeconfigFile)
			if _, ok := cfg.Contexts[oktetoContext]; !ok {
				return fmt.Errorf(errors.ErrKubernetesContextNotFound, oktetoContext, kubeconfigFile)
			}
			if err := saveContextConfigInFile(CurrentStore); err != nil {
				return err
			}
		}
		octxStore.CurrentContext = oktetoContext
	}

	if namespace != "" {
		octx := Context()
		octx.Namespace = namespace
	}

	return nil
}

func HasBeenLogged(oktetoURL string) bool {
	octxStore := ContextStore()
	_, ok := octxStore.Contexts[oktetoURL]
	return ok
}

func UpdateOktetoClusterContext(name string, u *User, namespace string, cfg *clientcmdapi.Config) error {
	if contextExists() {
		CurrentStore = ContextStore()
	} else {
		CurrentStore = &OktetoContextStore{
			Contexts: map[string]*OktetoContext{},
		}
	}

	kubeconfigBase64 := encodeOktetoKubeconfig(cfg)
	certificate := u.Certificate
	if certificate != "" {
		certificate = base64.StdEncoding.EncodeToString([]byte(u.Certificate))
	}
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:            name,
		UserID:          u.ID,
		Username:        u.ExternalID,
		Token:           u.Token,
		Namespace:       namespace,
		GlobalNamespace: u.GlobalNamespace,
		Kubeconfig:      kubeconfigBase64,
		Buildkit:        u.Buildkit,
		Registry:        u.Registry,
		Certificate:     certificate,
	}

	CurrentStore.CurrentContext = name
	return saveContextConfigInFile(CurrentStore)
}

func SaveOktetoClusterContext(name string, u *User, namespace string, cfg *clientcmdapi.Config) error {
	if contextExists() {
		CurrentStore = ContextStore()
	} else {
		CurrentStore = &OktetoContextStore{
			Contexts: map[string]*OktetoContext{},
		}
	}

	kubeconfigBase64 := ""
	if cfg != nil {
		kubeconfigBase64 = encodeOktetoKubeconfig(cfg)
	}
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:            name,
		UserID:          u.ID,
		Username:        u.ExternalID,
		Token:           u.Token,
		Namespace:       namespace,
		GlobalNamespace: u.GlobalNamespace,
		Kubeconfig:      kubeconfigBase64,
		Buildkit:        u.Buildkit,
		Registry:        u.Registry,
		Certificate:     u.Certificate,
	}

	CurrentStore.CurrentContext = name
	return saveContextConfigInFile(CurrentStore)
}

func SaveKubernetesClusterContext(name, namespace string, cfg *clientcmdapi.Config, buildkitURL string) error {
	var err error
	if contextExists() {
		CurrentStore = ContextStore()
		if err != nil {
			log.Errorf("bad contexts, re-initializing: %s", err)
		}
	} else {
		CurrentStore = &OktetoContextStore{
			Contexts: map[string]*OktetoContext{},
		}
	}

	kubeconfigBase64 := encodeOktetoKubeconfig(cfg)
	CurrentStore.Contexts[name] = &OktetoContext{
		Name:       name,
		Namespace:  namespace,
		Kubeconfig: kubeconfigBase64,
		Buildkit:   buildkitURL,
	}

	CurrentStore.CurrentContext = name
	return saveContextConfigInFile(CurrentStore)
}

func saveContextConfigInFile(c *OktetoContextStore) error {
	marshalled, err := json.MarshalIndent(c, "", "\t")
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

	if err := ioutil.WriteFile(contextConfigPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save context: %s", err)
	}

	return nil
}

//SetKubeContext updates the current context of a kubeconfig file
func SetKubeContext(cred *Credential, kubeConfigPath, namespace, userName, clusterName string) error {
	cfg := client.GetKubeconfig(kubeConfigPath)
	if cfg == nil {
		cfg = client.CreateKubeconfig()
	}

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

	return clientcmd.WriteToFile(*cfg, kubeConfigPath)
}

func encodeOktetoKubeconfig(cfg *clientcmdapi.Config) string {
	currentCluster := cfg.Contexts[cfg.CurrentContext].Cluster
	for name, cluster := range cfg.Clusters {
		if name == currentCluster {
			cluster.LocationOfOrigin = ""
			cfg.Clusters = map[string]*clientcmdapi.Cluster{name: cluster}
			break
		}
	}
	currentAuthInfo := cfg.Contexts[cfg.CurrentContext].AuthInfo
	for name, authInfo := range cfg.AuthInfos {
		if name == currentAuthInfo {
			authInfo.LocationOfOrigin = ""
			cfg.AuthInfos = map[string]*clientcmdapi.AuthInfo{name: authInfo}
			break
		}
	}
	for name, context := range cfg.Contexts {
		if name == cfg.CurrentContext {
			context.LocationOfOrigin = ""
			cfg.Contexts = map[string]*clientcmdapi.Context{name: context}
			break
		}
	}

	bytes, err := json.Marshal(cfg)
	if err != nil {
		log.Fatalf("error marsahiling kubeconfig: %v", err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func GetK8sClient() (*kubernetes.Clientset, *rest.Config, error) {
	octx := Context()
	kubeconfigBytes, err := base64.StdEncoding.DecodeString(octx.Kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf(errors.ErrCorruptedOktetoContexts, config.GetOktetoHome())
	}
	var cfg clientcmdapi.Config
	if err := json.Unmarshal(kubeconfigBytes, &cfg); err != nil {
		return nil, nil, err
	}
	kubeconfigFile := config.GetOktetoContextKubeconfigPath()
	if err := client.WriteKubeconfig(&cfg, kubeconfigFile); err != nil {
		return nil, nil, err
	}
	return client.Get(kubeconfigFile)
}

// GetSanitizedUsername returns the username of the authenticated user sanitized to be DNS compatible
func GetSanitizedUsername() string {
	octx := Context()
	return reg.ReplaceAllString(strings.ToLower(octx.Username), "-")
}
