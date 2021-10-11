package deploy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type kubeConfig struct{}

func newKubeConfig() *kubeConfig {
	return &kubeConfig{}
}

func (k *kubeConfig) Read() (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", k.getCMDAPIConfig)
}

func (k *kubeConfig) Modify(ctx context.Context, port int, sessionToken string) error {
	clientCfg, err := k.getCMDAPIConfig()
	if err != nil {
		return err
	}

	// Retrieve the auth info for the current context and change the bearer token to validate the request in our proxy
	authInfo := clientCfg.AuthInfos[clientCfg.Contexts[clientCfg.CurrentContext].AuthInfo]
	// Setting the token with the proxy session token
	authInfo.Token = sessionToken
	// Retrieve cluster info for current context
	clusterInfo := clientCfg.Clusters[clientCfg.Contexts[clientCfg.CurrentContext].Cluster]

	// Change server to our proxy
	clusterInfo.Server = fmt.Sprintf("https://localhost:%d", port)
	// Set the certificate authority to talk with the proxy
	clusterInfo.CertificateAuthorityData = cert

	// Save on disk the config changes
	if err := clientcmd.WriteToFile(*clientCfg, tempKubeConfig); err != nil {
		log.Errorf("could not modify the k8s config: %s", err)
		return err
	}
	return nil
}

func (k *kubeConfig) getCMDAPIConfig() (*clientcmdapi.Config, error) {
	kubeconfigBytes, err := base64.StdEncoding.DecodeString(okteto.Context().Kubeconfig)
	if err != nil {
		return nil, err
	}

	var clientCfg clientcmdapi.Config
	if err := json.Unmarshal(kubeconfigBytes, &clientCfg); err != nil {
		return nil, err
	}

	return &clientCfg, nil
}
