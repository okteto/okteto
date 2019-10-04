package client

import (
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var client *kubernetes.Clientset
var config *rest.Config
var namespace string

//GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal() (*kubernetes.Clientset, *rest.Config, string, error) {
	if client == nil {
		var err error

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return nil, nil, "", err
		}

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, "", err
		}
		config.Timeout = 5 * time.Second

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, nil, "", err
		}
	}
	return client, config, namespace, nil
}
