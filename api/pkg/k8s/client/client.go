package client

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/okteto/app/api/pkg/log"
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
func Get() *kubernetes.Clientset {
	if client == nil {
		log.Infof("initializing kubernetes client")
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalf("failed to get kubernetes config: %s", err)
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("failed to initialize kubernetes client: %s", err)
		}
	}
	return client
}
