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
	client        *kubernetes.Clientset
	config        *rest.Config
	namespace     string
	context       string
	localClusters = []string{"127.", "172.", "192.", "169.", model.Localhost, "::1", "fe80::", "fc00::"}
)

//GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal(k8sContext string) (*kubernetes.Clientset, *rest.Config, string, error) {
	if client == nil {
		var err error

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{
				CurrentContext: k8sContext,
				ClusterInfo:    clientcmdapi.Cluster{Server: ""},
			},
		)

		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return nil, nil, "", err
		}

		rawConfig, err := clientConfig.RawConfig()
		if err != nil {
			return nil, nil, "", err
		}
		context = rawConfig.CurrentContext

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, "", err
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, nil, "", err
		}

		setAnalytics(context, config.Host)
	}

	return client, config, namespace, nil
}

//Reset cleans the cached client
func Reset() {
	client = nil
	config = nil
	namespace = ""
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
