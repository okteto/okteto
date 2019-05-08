package client

import (
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var namespace string
var client *kubernetes.Clientset

//GetOktetoNamespace returns the namespace where okteto app is running
func GetOktetoNamespace() string {
	if namespace == "" {
		b, err := ioutil.ReadFile(namespaceFile)
		if err != nil {
			fmt.Println("Error reading master namespace")
			os.Exit(1)
		}
		namespace = string(b)
	}
	return namespace
}

//Get returns the k8s client
func Get() (*kubernetes.Clientset, error) {
	if client == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}
