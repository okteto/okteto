package client

import (
	"fmt"
	"os"

	"bitbucket.org/okteto/okteto/backend/model"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//Get returns the k8 client for a given provider
func Get(p *model.Provider) (*kubernetes.Clientset, error) {
	configPath, err := getConfig(p)
	if err != nil {
		return nil, err
	}
	defer os.Remove(configPath)

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("Error reading k8 config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Error creating k8 client: %s", err)
	}
	return clientset, nil
}
