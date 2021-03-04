// Copyright 2020 The Okteto Authors
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
	"context"
	"log"
	"net/url"
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
)

var (
	config           *rest.Config
	clientConfig     clientcmd.ClientConfig
	currentNamespace string
	currentContext   string
	localClusters    = []string{"127.", "172.", "192.", "169.", model.Localhost, "::1", "fe80::", "fc00::"}
)

//GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal() (*kubernetes.Clientset, *rest.Config, error) {
	return GetLocalWithContext("")
}

//GetLocalWithContext returns a kubernetes client for a given context. It will detect if KUBECONFIG is defined.
func GetLocalWithContext(currentContext string) (*kubernetes.Clientset, *rest.Config, error) {
	if config == nil {
		clientConfig = getClientConfig(currentContext)
		if okteto.GetClusterContext() == currentContext {
			ctx := context.Background()
			namespace := GetCurrentNamespace(currentContext)
			go okteto.RefreshOktetoKubeconfig(ctx, namespace)
		}

		var err error
		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, err
		}

		config.Wrap(refreshCredentialsFn)

		setAnalytics(currentContext, config.Host)

	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return client, config, nil
}

func getClientConfig(k8sContext string) clientcmd.ClientConfig {
	if clientConfig != nil {
		return clientConfig
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{
			CurrentContext: k8sContext,
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
		},
	)
}

//GetCurrentContext sets the client config for the context in use
func GetCurrentContext() string {
	if currentContext != "" {
		return currentContext
	}
	cc := getClientConfig("")
	rawConfig, err := cc.RawConfig()
	if err != nil {
		log.Fatalf("error accessing you kubeconfig file: %s", err.Error())
	}
	currentContext = rawConfig.CurrentContext
	return currentContext
}

//GetCurrentNamespace returns the name of the namespace in use by a given context
func GetCurrentNamespace(k8sContext string) string {
	if currentNamespace != "" {
		return currentNamespace
	}
	namespace, _, err := getClientConfig(k8sContext).Namespace()
	if err != nil {
		log.Fatalf("error accessing you kubeconfig file: %s", err.Error())
	}
	currentNamespace = namespace
	return currentNamespace
}

//Reset cleans the cached client
func Reset() {
	config = nil
	clientConfig = nil
	currentNamespace = ""
	currentContext = ""
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
