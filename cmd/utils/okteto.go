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

package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
)

func SetOktetoUsernameEnv() error {
	if username := os.Getenv("OKTETO_USERNAME"); username == "" {
		username := strings.ToLower(okteto.GetUsername())
		if err := os.Setenv("OKTETO_USERNAME", username); err != nil {
			return err
		}
	}
	return nil
}

func HasAccessToNamespace(ctx context.Context, namespace string) (bool, error) {
	nList, err := okteto.ListNamespaces(ctx)
	if err != nil {
		return false, err
	}

	for i := range nList {
		if nList[i].ID == namespace {
			return true, nil
		}
	}
	return false, nil
}

func InitOktetoContext() error {
	if okteto.ContextExists() {
		return nil
	}

	defer os.RemoveAll(config.GetTokenPath())

	kubeconfigPath := config.GetKubeconfigPath()
	currentContext := os.Getenv(client.OktetoContextVariableName)
	cfg := client.GetOrCreateKubeconfig(kubeconfigPath)
	if currentContext == "" {
		currentContext = client.GetCurrentContext(kubeconfigPath)
	}
	if _, ok := cfg.Clusters[currentContext]; !ok {
		return fmt.Errorf("current kubernetes context '%s' doesn't exist in '%s'", currentContext, config.GetKubeconfigPath())
	}

	if okteto.IsAuthenticated() && okteto.GetClusterContext() == currentContext {
		if cfg.Clusters[currentContext].Extensions == nil {
			cfg.Clusters[currentContext].Extensions = map[string]runtime.Object{}
		}
		cfg.Clusters[currentContext].Extensions[model.OktetoExtension] = nil

		if err := clientcmd.WriteToFile(*cfg, config.GetKubeconfigPath()); err != nil {
			return fmt.Errorf("error updating your KUBECONFIG file '%s': %s", config.GetKubeconfigPath(), err)
		}

		token, err := okteto.GetToken()
		if err != nil {
			return fmt.Errorf("error accessing okteto token '%s': %s", config.GetTokenPath(), err)
		}
		if err := okteto.SetCurrentContext(token.URL, token.ID, token.Username, token.Token, cfg.Contexts[currentContext].Namespace, "kubeconfig", token.Buildkit, token.URL); err != nil {
			return fmt.Errorf("error configuring okteto context: %s", err)
		}
		log.Information("Current context: %s\n    Run 'okteto context' to configure your context", token.URL)

		return nil
	}

	if err := okteto.SetCurrentContext(currentContext, "", "", "", cfg.Contexts[currentContext].Namespace, "kubeconfig", "", ""); err != nil {
		return fmt.Errorf("error configuring okteto context: %s", err)
	}
	log.Information("Current context: %s\n    Run 'okteto context' to configure your context", currentContext)

	return nil
}
