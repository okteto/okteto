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
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	oktetoClusterType = "okteto"
	localClusterType  = "local"
	remoteClusterType = "remote"
	//OktetoContextVariableName defines the kubeconfig context of okteto commands
	OktetoContextVariableName = "OKTETO_CONTEXT"
)

var (
	sessionContext string
	localClusters  = []string{"127.", "172.", "192.", "169.", model.Localhost, "::1", "fe80::", "fc00::"}
)

// GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal() (*kubernetes.Clientset, *rest.Config, error) {
	return GetLocalWithContext(os.Getenv(OktetoContextVariableName))
}

// GetLocalWithContext returns a kubernetes client for a given context. It will detect if KUBECONFIG is defined.
func GetLocalWithContext(thisContext string) (*kubernetes.Clientset, *rest.Config, error) {
	thisContext = GetSessionContext(thisContext)
	clientConfig := getClientConfig(thisContext)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	config.Timeout = getKubernetesTimeout()

	setAnalytics(sessionContext, config.Host)

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}

func getClientConfig(k8sContext string) clientcmd.ClientConfig {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{
			CurrentContext: k8sContext,
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
		},
	)
}

// GetSessionContext sets the client config for the context in use
func GetSessionContext(k8sContext string) string {
	if k8sContext != "" {
		return k8sContext
	}
	if sessionContext != "" {
		return sessionContext
	}
	cc := getClientConfig("")
	rawConfig, err := cc.RawConfig()
	if err != nil {
		log.Fatalf("error accessing you kubeconfig file: %s", err.Error())
	}
	sessionContext = rawConfig.CurrentContext
	return sessionContext
}

// GetContextNamespace returns the name of the namespace in use by a given context
func GetContextNamespace(k8sContext string) string {
	if k8sContext == "" {
		k8sContext = os.Getenv(OktetoContextVariableName)
	}
	namespace, _, err := getClientConfig(k8sContext).Namespace()
	if err != nil {
		log.Fatalf("error accessing you kubeconfig file: %s", err.Error())
	}
	return namespace
}

// Reset cleans the cached client
func Reset() {
	sessionContext = ""
}

// InCluster returns true if Okteto is running on a Kubernetes cluster
func InCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}

func setAnalytics(clusterContext, clusterHost string) {
	if okteto.GetClusterContext() == clusterContext {
		analytics.SetClusterType(oktetoClusterType)
		analytics.SetClusterContext(clusterContext)
		return
	}

	u, err := url.Parse(clusterHost)
	host := ""
	if err == nil {
		host = u.Hostname()
	} else {
		host = clusterHost
	}
	for _, l := range localClusters {
		if strings.HasPrefix(host, l) {
			analytics.SetClusterType(localClusterType)
			return
		}
	}
	analytics.SetClusterType(remoteClusterType)
}
