package client

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/app/cli/pkg/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Get returns a kubernetes client.
func Get() (*kubernetes.Clientset, *rest.Config, error) {
	home := config.GetHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if len(kubeconfigEnv) > 0 {
		kubeconfig = kubeconfigEnv
	}

	_, err := os.Stat(kubeconfig)
	if err != nil && os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("Kubernetes configuration does not exit at %s", kubeconfig)
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

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
