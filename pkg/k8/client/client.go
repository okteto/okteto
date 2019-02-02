package client

import (
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Get returns a kubernetes client.
// If namespace is empty, it will use the default namespace configured.
// If path is empty, it will use the default path configuration
func Get(namespace, configPath string) (string, *kubernetes.Clientset, *rest.Config, string, error) {
	log.Debugf("reading kubernetes configuration from %s", configPath)

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: configPath},
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
