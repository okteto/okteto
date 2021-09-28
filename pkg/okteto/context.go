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

type ContextStore struct {
	Contexts       map[string]Context `json:"contexts"`
	CurrentContext string             `json:"current-context"`
}

// Context contains the information related to an okteto context
type Context struct {
	Name        string `json:"name,omitempty"`
	ID          string `json:"id,omitempty"`
	Username    string `json:"username,omitempty"`
	Token       string `json:"token,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Kubeconfig  string `json:"kubeconfig,omitempty"`
	Buildkit    string `json:"buildkit,omitempty"`
	Registry    string `json:"registry,omitempty"`
	Certificate string `json:"certificate,omitempty"`
}

func InitContext(ctx context.Context) error {
	if contextExists() {
		return nil
	}

	defer os.RemoveAll(config.GetTokenPath())

	kubeconfigPath := config.GetKubeconfigPath()
	currentContext := os.Getenv(client.OktetoContextVariableName)
	cfg := client.GetKubeconfig(kubeconfigPath)
	if currentContext == "" {
		currentContext = client.GetCurrentContext(kubeconfigPath)
	}

	kubeconfigFile := config.GetKubeconfigPath()
	if IsAuthenticated() {
		token, err := GetToken()
		if err != nil {
			log.Infof("error accessing okteto token '%s': %v", config.GetTokenPath(), err)
		}
		certBytes, errCert := os.ReadFile(config.GetCertificatePath())
		if err != nil {
			log.Infof("error reading current okteto certificate: %v", err)
		}

		if GetKubernetesContextFromToken() == currentContext && errCert == nil {
			if cfg.Clusters[currentContext].Extensions == nil {
				cfg.Clusters[currentContext].Extensions = map[string]runtime.Object{}
			}
			cfg.Clusters[currentContext].Extensions[model.OktetoExtension] = nil

			if err := clientcmd.WriteToFile(*cfg, kubeconfigFile); err != nil {
				return fmt.Errorf("error updating your KUBECONFIG file '%s': %v", kubeconfigFile, err)
			}

			cfg := client.GetKubeconfig(kubeconfigFile)
			if err == nil {
				if err := SetCurrentContext(token.URL, token.ID, token.Username, token.Token, cfg.Contexts[currentContext].Namespace, cfg, token.Buildkit, token.URL, string(certBytes)); err != nil {
					return fmt.Errorf("error configuring okteto context: %v", err)
				}
				log.Information("Current context: %s\n    Run 'okteto context' to configure your context", token.URL)
				return nil
			}
			log.Infof("error reading current okteto certificate: %v", err)
		}

		if cfg == nil && errCert == nil {
			cred, err := GetCredentials(ctx)
			if err == nil {
				oktetoContext := GetKubernetesContextFromToken()

				if err := SetKubeconfig(cred, kubeconfigFile, cred.Namespace, token.ID, oktetoContext); err != nil {
					return fmt.Errorf("error updating kubernetes context: %v", err)
				}

				cfg := client.GetKubeconfig(kubeconfigFile)
				if err := SetCurrentContext(token.URL, token.ID, token.Username, token.Token, cred.Namespace, cfg, token.Buildkit, token.URL, string(certBytes)); err != nil {
					return fmt.Errorf("error configuring okteto context: %v", err)
				}

				return nil
			}
			log.Infof("error refreshing okteto credentials: %v", err)
		}
	}

	if cfg == nil {
		return errors.ErrNoActiveOktetoContexts
	}

	if _, ok := cfg.Clusters[currentContext]; !ok {
		return fmt.Errorf("current kubernetes context '%s' doesn't exist in '%s'", currentContext, kubeconfigFile)
	}

	if err := SetCurrentContext(currentContext, "", "", "", cfg.Contexts[currentContext].Namespace, cfg, "", "", ""); err != nil {
		return fmt.Errorf("error configuring okteto context: %v", err)
	}
	log.Information("Current context: %s\n    Run 'okteto context' if you need to change your context", currentContext)

	return nil
}

//contextExists checks if an okteto context has been created
func contextExists() bool {
	if _, err := os.Stat(config.GetOktetoContextFolder()); !os.IsNotExist(err) {
		return true
	}
	return false
}

func IsOktetoContext(name string) bool {
	u, err := url.Parse(name)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func IsOktetoCurrentContext() bool {
	return IsOktetoContext(GetCurrentContext())
}

//TODO: read just once
func GetContexts() (*ContextStore, error) {
	p := config.GetOktetoContextsConfigPath()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, errors.ErrNoActiveOktetoContexts
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // Force errors

	ctxConfig := &ContextStore{}
	if err := dec.Decode(&ctxConfig); err != nil {
		return nil, err
	}

	return ctxConfig, nil
}

func GetCurrentContext() string {
	c, err := GetContexts()
	if err != nil {
		return ""
	}
	return c.CurrentContext
}

func SetCurrentContext(name, userID, username, token, namespace string, cfg *clientcmdapi.Config, buildkitURL, registryURL, certificate string) error {
	var err error
	var cc *ContextStore
	if contextExists() {
		cc, err = GetContexts()
		if err != nil {
			log.Infof("bad contexts, re-initializing: %s", err)
		}
	} else {
		cc = &ContextStore{
			Contexts: make(map[string]Context),
		}
	}

	kubeconfigBase64 := encodeOktetoKubeconfig(cfg)
	cc.Contexts[name] = Context{
		Name:        name,
		ID:          userID,
		Username:    username,
		Token:       token,
		Namespace:   namespace,
		Kubeconfig:  kubeconfigBase64,
		Buildkit:    buildkitURL,
		Registry:    registryURL,
		Certificate: base64.StdEncoding.EncodeToString([]byte(certificate)),
	}

	cc.CurrentContext = name

	return saveContextConfigInFile(cc)
}

func saveContextConfigInFile(c *ContextStore) error {
	marshalled, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		log.Infof("failed to marshal context: %s", err)
		return fmt.Errorf("failed to generate your context")
	}

	contextFolder := config.GetOktetoContextFolder()
	if err := os.MkdirAll(contextFolder, 0700); err != nil {
		log.Fatalf("failed to create %s: %s", contextFolder, err)
	}

	contextConfigPath := config.GetOktetoContextsConfigPath()
	if _, err := os.Stat(contextConfigPath); err == nil {
		err = os.Chmod(contextConfigPath, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change context permissions: %s", err)
		}
	}

	if err := ioutil.WriteFile(contextConfigPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save context: %s", err)
	}

	currentToken = nil
	return nil
}

//SetKubeconfig updates the current context of a kubeconfig file
func SetKubeconfig(cred *Credential, kubeConfigPath, namespace, userName, clusterName string) error {
	cfg := client.GetKubeconfig(kubeConfigPath)
	if cfg == nil {
		cfg = client.CreateKubeconfig()
	}

	// create cluster
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}
	if cluster.Extensions == nil {
		cluster.Extensions = map[string]runtime.Object{}
	}
	cluster.Extensions[model.OktetoExtension] = nil

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
	occ, err := GetContexts()
	if err != nil {
		return nil, nil, err
	}
	//TODO: out of bound index
	kubeconfigBase64 := occ.Contexts[occ.CurrentContext].Kubeconfig
	kubeconfigBytes, err := base64.StdEncoding.DecodeString(kubeconfigBase64)
	if err != nil {
		return nil, nil, err
	}
	var cfg clientcmdapi.Config
	if err := json.Unmarshal(kubeconfigBytes, &cfg); err != nil {
		return nil, nil, err
	}
	kubeconfigFile := config.GetOktetoContextKubeconfigPath()
	if err := client.SetKubeconfig(&cfg, kubeconfigFile); err != nil {
		return nil, nil, err
	}
	return client.GetLocal()
}
