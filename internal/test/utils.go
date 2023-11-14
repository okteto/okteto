// Copyright 2023 The Okteto Authors
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

package test

import (
	"os"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type KubeconfigFields struct {
	CurrentContext string
	ClusterCert    string
	Name           []string
	Namespace      []string
}

func CreateKubeconfig(kubeconfigFields KubeconfigFields) (string, error) {
	dir, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}

	os.Setenv(constants.KubeConfigEnvVar, dir.Name())

	contexts := make(map[string]*clientcmdapi.Context)
	for idx := range kubeconfigFields.Name {
		contexts[kubeconfigFields.Name[idx]] = &clientcmdapi.Context{Namespace: kubeconfigFields.Namespace[idx]}
	}
	cfg := &clientcmdapi.Config{
		Contexts:       contexts,
		CurrentContext: kubeconfigFields.CurrentContext,
		Clusters: map[string]*clientcmdapi.Cluster{
			kubeconfigFields.CurrentContext: {
				CertificateAuthorityData: []byte(kubeconfigFields.ClusterCert),
			},
		},
	}
	if err := kubeconfig.Write(cfg, dir.Name()); err != nil {
		return "", err
	}
	return dir.Name(), nil
}
