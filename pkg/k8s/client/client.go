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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var client *kubernetes.Clientset
var config *rest.Config
var namespace string

//GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal(context string) (*kubernetes.Clientset, *rest.Config, string, error) {
	if client == nil {
		var err error

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{
				CurrentContext: context,
				ClusterInfo:    clientcmdapi.Cluster{Server: ""},
			},
		)

		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return nil, nil, "", err
		}

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, "", err
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, nil, "", err
		}
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
