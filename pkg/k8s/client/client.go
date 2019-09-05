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

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, nil, "", err
		}
	}
	return client, config, namespace, nil
}

//IsOktetoCloud returns if the kubernetes cluster is Okteto Cloud
func IsOktetoCloud() bool {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

	c, err := clientConfig.RawConfig()
	if err != nil {
		return false
	}
	return c.CurrentContext == "cloud_okteto_com-context"
}
