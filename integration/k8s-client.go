package integration

import (
	"github.com/okteto/okteto/pkg/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// K8sClient returns a kubernetes client for current KUBECONFIG
func K8sClient() (*kubernetes.Clientset, *rest.Config, error) {
	clientConfig := GetClientConfig(config.GetKubeconfigPath(), "")

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}

func GetClientConfig(kubeconfigPaths []string, kubeContext string) clientcmd.ClientConfig {

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.Precedence = kubeconfigPaths

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{
			CurrentContext: kubeContext,
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
		},
	)
}
