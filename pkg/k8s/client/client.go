package client

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//GetLocal returns a kubernetes client with the local configuration. It will detect if KUBECONFIG is defined.
func GetLocal() (*kubernetes.Clientset, *rest.Config, string, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
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

//SetKubeConfig update a kubeconfig file with okteto cluster credentials
func SetKubeConfig(filename, namespace string) error {
	oktetoURL := okteto.GetURLWithUnderscore()
	clusterName := fmt.Sprintf("%s-cluster", oktetoURL)
	userName := fmt.Sprintf("%s-user", oktetoURL)
	contextName := fmt.Sprintf("%s-context", oktetoURL)

	var cfg *clientcmdapi.Config
	cred, err := okteto.GetCredentials(namespace)
	if err != nil {
		return err
	}

	_, err = os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = clientcmdapi.NewConfig()
		} else {
			return err
		}
	} else {
		cfg, err = clientcmd.LoadFromFile(filename)
		if err != nil {
			return err
		}
	}

	//create cluster
	cluster, ok := cfg.Clusters[clusterName]
	if !ok {
		cluster = clientcmdapi.NewCluster()
	}
	cluster.CertificateAuthorityData = []byte(cred.Certificate)
	cluster.Server = cred.Server
	cfg.Clusters[clusterName] = cluster

	//create user
	user, ok := cfg.AuthInfos[userName]
	if !ok {
		user = clientcmdapi.NewAuthInfo()
	}
	user.Token = cred.Token
	cfg.AuthInfos[userName] = user

	//create context
	context, ok := cfg.Contexts[contextName]
	if !ok {
		context = clientcmdapi.NewContext()
	}
	context.Cluster = clusterName
	context.AuthInfo = userName
	context.Namespace = cred.Namespace
	cfg.Contexts[contextName] = context

	cfg.CurrentContext = contextName

	if err := clientcmd.WriteToFile(*cfg, filename); err != nil {
		return err
	}
	return nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
