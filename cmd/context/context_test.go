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

package context

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type kubeconfigFields struct {
	Name           []string
	Namespace      []string
	CurrentContext string
}

func Test_initFromDeprecatedToken(t *testing.T) {

	var tests = []struct {
		name          string
		tokenUrl      string
		kubeconfigCtx kubeconfigFields
	}{
		{
			name:     "token-create-kubeconfig",
			tokenUrl: "https://cloud.okteto.com",
			kubeconfigCtx: kubeconfigFields{
				Name:           []string{"test"},
				Namespace:      []string{"test"},
				CurrentContext: "cloud_okteto_com",
			},
		},
		{
			name:     "token-token-but-not-in-kubeconfig",
			tokenUrl: "https://cloud.okteto.com",
			kubeconfigCtx: kubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenPath, err := createDeprecatedToken(tt.tokenUrl)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tokenPath)

			kubepath, err := createKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(kubepath)
			okteto.InitContextWithDeprecatedToken()
			if okteto.ContextStore().CurrentContext == "" {
				t.Fatal("Not initialized")
			}
		})
	}
}

func createDeprecatedToken(url string) (string, error) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}
	os.Setenv("OKTETO_FOLDER", dir)
	token := &okteto.Token{
		URL:       url,
		Buildkit:  "buildkit",
		Registry:  "registry",
		ID:        "id",
		Username:  "username",
		Token:     "token",
		MachineID: "machine-id",
	}
	marshalled, err := json.MarshalIndent(token, "", "\t")
	if err != nil {
		return "", fmt.Errorf("failed to generate analytics file: %s", err)
	}

	if err := os.WriteFile(config.GetTokenPathDeprecated(), marshalled, 0600); err != nil {
		return "", fmt.Errorf("couldn't save analytics: %s", err)
	}
	if _, err := os.Stat(config.GetTokenPathDeprecated()); err != nil {
		return "", fmt.Errorf("Not created correctly")
	}
	if err := os.WriteFile(config.GetCertificatePath(), []byte("a"), 0600); err != nil {
		return "", fmt.Errorf("couldn't save analytics: %s", err)
	}

	return config.GetTokenPathDeprecated(), nil
}

func createKubeconfig(kubeconfigFields kubeconfigFields) (string, error) {
	dir, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}

	os.Setenv("KUBECONFIG", dir.Name())

	contexts := make(map[string]*clientcmdapi.Context)
	for idx := range kubeconfigFields.Name {
		contexts[kubeconfigFields.Name[idx]] = &clientcmdapi.Context{Namespace: kubeconfigFields.Namespace[idx]}
	}
	cfg := &clientcmdapi.Config{
		Contexts:       contexts,
		CurrentContext: kubeconfigFields.CurrentContext,
	}
	if err := kubeconfig.Write(cfg, dir.Name()); err != nil {
		return "", err
	}
	return dir.Name(), nil
}
