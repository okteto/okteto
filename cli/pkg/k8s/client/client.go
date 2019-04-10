package client

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Get returns a kubernetes client.
func Get(configB64 string) (*kubernetes.Clientset, *rest.Config, error) {
	configFile, err := ioutil.TempFile("", "k8-config")
	if err != nil {
		return nil, nil, fmt.Errorf("Error creating tmp file: %s", err)
	}
	configValue, err := base64.StdEncoding.DecodeString(configB64)
	if err != nil {
		return nil, nil, fmt.Errorf("Error decoding credentials: %s", err)
	}

	if err := ioutil.WriteFile(configFile.Name(), []byte(configValue), 0400); err != nil {
		return nil, nil, fmt.Errorf("Error writing credentials: %s", err)
	}
	kubeconfig := configFile.Name()

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
