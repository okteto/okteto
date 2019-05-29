package client

import (
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/okteto/app/cli/pkg/okteto"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//Get returns a kubernetes client.
func Get() (*kubernetes.Clientset, *rest.Config, string, error) {
	configFile, err := ioutil.TempFile("", "k8-config")
	if err != nil {
		return nil, nil, "", err
	}
	kubeconfig := configFile.Name()
	if err := SetKubeConfig(kubeconfig, ""); err != nil {
		return nil, nil, "", err
	}

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

//SetKubeConfig update a kubeconfig file with okteto cluster credentials
func SetKubeConfig(filename, space string) error {
	oktetoURL := okteto.GetURLWithUnderscore()
	clusterName := fmt.Sprintf("%s-cluster", oktetoURL)
	userName := fmt.Sprintf("%s-user", oktetoURL)
	contextName := fmt.Sprintf("%s-context", oktetoURL)

	var cfg *clientcmdapi.Config
	cred, err := okteto.GetCredentials(space)
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
