// Copyright 2022 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"fmt"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//KubeConfig refers to a KubeConfig object
type KubeConfig struct{}

//NewKubeConfig creates a new kubeconfig
func NewKubeConfig() *KubeConfig {
	return &KubeConfig{}
}

//Read reads a kubeconfig from an apiConfig
func (k *KubeConfig) Read() (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", k.GetCMDAPIConfig)
}

//Modify modifies the kubeconfig object to inject the proxy
func (k *KubeConfig) Modify(port int, sessionToken, destKubeconfigFile string) error {
	clientCfg, err := k.GetCMDAPIConfig()
	if err != nil {
		return err
	}

	// We should change only the config for the proxy, not the one in Context.Cfg
	proxyCfg := clientCfg.DeepCopy()

	// Retrieve the auth info for the current context and change the bearer token to validate the request in our proxy
	authInfo := proxyCfg.AuthInfos[proxyCfg.Contexts[proxyCfg.CurrentContext].AuthInfo]
	// Setting the token with the proxy session token
	authInfo.Token = sessionToken
	// Retrieve cluster info for current context
	clusterInfo := proxyCfg.Clusters[proxyCfg.Contexts[proxyCfg.CurrentContext].Cluster]

	// Change server to our proxy
	clusterInfo.Server = fmt.Sprintf("https://localhost:%d", port)
	// Set the certificate authority to talk with the proxy
	clusterInfo.CertificateAuthorityData = cert

	// Save on disk the config changes
	if err := clientcmd.WriteToFile(*proxyCfg, destKubeconfigFile); err != nil {
		oktetoLog.Errorf("could not modify the k8s config: %s", err)
		return err
	}
	return nil
}

func (*KubeConfig) GetCMDAPIConfig() (*clientcmdapi.Config, error) {
	if okteto.Context().Cfg == nil {
		return nil, fmt.Errorf("okteto context not initialized")
	}

	return okteto.Context().Cfg, nil
}
