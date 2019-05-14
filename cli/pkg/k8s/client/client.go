package client

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"github.com/okteto/app/cli/pkg/okteto"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Get returns a kubernetes client.
func Get() (*kubernetes.Clientset, *rest.Config, string, error) {
	configB64, err := okteto.GetK8sB64Config("")
	if err != nil {
		return nil, nil, "", err
	}

	configFile, err := ioutil.TempFile("", "k8-config")
	if err != nil {
		return nil, nil, "", fmt.Errorf("Error creating tmp file: %s", err)
	}
	configValue, err := base64.StdEncoding.DecodeString(configB64)
	if err != nil {
		return nil, nil, "", fmt.Errorf("Error decoding credentials: %s", err)
	}

	if err := ioutil.WriteFile(configFile.Name(), []byte(configValue), 0400); err != nil {
		return nil, nil, "", fmt.Errorf("Error writing credentials: %s", err)
	}
	kubeconfig := configFile.Name()

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, nil, "", err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, "", err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, "", err
	}

	return client, config, namespace, nil
}
