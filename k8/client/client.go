package client

import (
	"os"
	"path"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Get returns a kubernetes client
func Get() (string, *kubernetes.Clientset, *rest.Config, error) {
	home := os.Getenv("HOME")
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: path.Join(home, ".kube/config")},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return "", nil, nil, err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return "", nil, nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", nil, nil, err
	}

	return namespace, client, config, nil
}
