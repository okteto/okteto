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
var userID string

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

//GetUserID returns a user info of the ccurrent context
func GetUserID() string {
	if userID == "" {
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

		c, err := clientConfig.RawConfig()
		if err != nil {
			return ""
		}
		ctx, ok := c.Contexts[c.CurrentContext]
		if !ok {
			return ""
		}
		userID = ctx.AuthInfo
	}
	return userID
}
