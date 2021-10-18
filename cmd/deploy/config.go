// Copyright 2021 The Okteto Authors
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

func (k *kubeConfig) Modify(port int, sessionToken, destKubeconfigFile string) error {
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
	if err := clientcmd.WriteToFile(*clientCfg, destKubeconfigFile); err != nil {
		log.Errorf("could not modify the k8s config: %s", err)
		return err
	}
	return nil
}

func (*kubeConfig) getCMDAPIConfig() (*clientcmdapi.Config, error) {
	if okteto.Context().Cfg == nil {
		return nil, fmt.Errorf("okteto context not initialized")
	}

	return okteto.Context().Cfg, nil
}
