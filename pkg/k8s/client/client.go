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

package client

import (
	"log"
	"os"

	"github.com/okteto/okteto/pkg/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	//OktetoContextVariableName defines the kubeconfig context of okteto commands
	OktetoContextVariableName = "OKTETO_CONTEXT"
)

// InCluster returns true if Okteto is running on a Kubernetes cluster
func InCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}

// GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal() (*kubernetes.Clientset, *rest.Config, error) {
	kubeconfigFile := config.GetOktetoContextKubeconfigPath()
	clientConfig := getClientConfig(kubeconfigFile, "")

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	config.Timeout = getKubernetesTimeout()

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}

func getClientConfig(kubeconfigPath, kubeContext string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeconfigPath

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{
			CurrentContext: kubeContext,
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
		},
	)
}

//CreateKubeconfig creates anew  kubeconfig file
func CreateKubeconfig() *clientcmdapi.Config {
	return clientcmdapi.NewConfig()
}

//GetKubeconfig retrieves a kubeconfig file
func GetKubeconfig(kubeconfigPath string) *clientcmdapi.Config {
	_, err := os.Stat(kubeconfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Fatalf("error accessing your KUBECONFIG file '%s': %v", kubeconfigPath, err)
	}

	cfg, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Fatalf("error accessing your KUBECONFIG file '%s': %v", kubeconfigPath, err)
	}

	return cfg
}

//SetKubeconfig stores a kubeconfig file
func SetKubeconfig(cfg *clientcmdapi.Config, kubeconfigPath string) error {
	return clientcmd.WriteToFile(*cfg, kubeconfigPath)
}

// GetCurrentContext returns the name of the current context
func GetCurrentContext(kubeconfigPath string) string {
	cfg := GetKubeconfig(kubeconfigPath)
	if cfg == nil {
		return ""
	}
	return cfg.CurrentContext
}

// GetCurrentNamespace returns the name of the namespace in use by a given context
func GetCurrentNamespace(kubeconfigPath, kubeContext string) string {
	cfg := GetKubeconfig(kubeconfigPath)
	if cfg == nil {
		return ""
	}
	if kubeContext == "" {
		kubeContext = cfg.CurrentContext
	}
	if currentContext, ok := cfg.Contexts[kubeContext]; ok {
		return currentContext.Namespace
	}
	return ""
}
