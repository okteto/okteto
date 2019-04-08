package client

import (
	"fmt"
	"os"
	"path/filepath"

	"cli/cnd/pkg/model"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// GetCurrentNamespace returns the current kubernetes namespace
func GetCurrentNamespace() (string, error) {
	n, _, _, _, err := Get("")
	return n, err
}

//Get returns a kubernetes client.
// If namespace is empty, it will use the default namespace configured.
// If path is empty, it will use the default path configuration
func Get(namespace string) (string, *kubernetes.Clientset, *rest.Config, string, error) {
	home := model.GetHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if len(kubeconfigEnv) > 0 {
		kubeconfig = kubeconfigEnv
	}

	_, err := os.Stat(kubeconfig)
	if err != nil && os.IsNotExist(err) {
		return "", nil, nil, "", fmt.Errorf("Kubernetes configuration does not exit at %s", kubeconfig)
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

	if namespace == "" {
		var err error
		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return "", nil, nil, "", err
		}
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return "", nil, nil, "", err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", nil, nil, "", err
	}

	rc, err := clientConfig.RawConfig()
	if err != nil {
		return "", nil, nil, "", err
	}

	return namespace, client, config, rc.CurrentContext, nil
}
