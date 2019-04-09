package client

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//Get returns the k8s client
func Get() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
