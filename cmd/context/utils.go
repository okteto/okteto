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

package context

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
)

type SelectItem struct {
	Name   string
	Enable bool
}

type ManifestOptions struct {
	Name       string
	Namespace  string
	Filename   string
	K8sContext string
}

func getKubernetesContextList(filterOkteto bool) []string {
	contextList := make([]string, 0)
	kubeconfigFile := config.GetKubeconfigPath()
	cfg := kubeconfig.Get(kubeconfigFile)
	if cfg == nil {
		return contextList
	}
	if !filterOkteto {
		for name := range cfg.Contexts {
			contextList = append(contextList, name)
		}
		return contextList
	}
	for name := range cfg.Contexts {
		if _, ok := cfg.Contexts[name].Extensions[constants.OktetoExtension]; ok && filterOkteto {
			continue
		}
		contextList = append(contextList, name)
	}
	return contextList
}

func getKubernetesContextNamespace(k8sContext string) string {
	kubeconfigFile := config.GetKubeconfigPath()
	cfg := kubeconfig.Get(kubeconfigFile)
	if cfg == nil {
		return ""
	}
	if cfg.Contexts[k8sContext].Namespace == "" {
		return "default"
	}
	return cfg.Contexts[k8sContext].Namespace
}

func isCreateNewContextOption(option string) bool {
	return option == newOEOption
}

func askForOktetoURL(message string) (string, error) {
	err := oktetoLog.Question(message)
	if err != nil {
		return "", err
	}
	var oktetoURL string
	fmt.Scanln(&oktetoURL)

	url, err := url.Parse(oktetoURL)
	if err != nil {
		return "", nil
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}
	return strings.TrimSuffix(url.String(), "/"), nil
}

func isValidCluster(cluster string) bool {
	for _, c := range getKubernetesContextList(false) {
		if cluster == c {
			return true
		}
	}
	return false
}

func LoadStackWithContext(ctx context.Context, name, namespace string, stackPaths []string, fs afero.Fs) (*model.Stack, error) {
	ctxResource, err := utils.LoadStackContext(stackPaths)
	if err != nil {
		if name == "" {
			return nil, err
		}
		ctxResource = &model.ContextResource{}
	}

	if err := ctxResource.UpdateNamespace(namespace); err != nil {
		return nil, err
	}

	ctxOptions := &Options{
		Context:   ctxResource.Context,
		Namespace: ctxResource.Namespace,
		Show:      true,
	}

	if err := NewContextCommand().Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	s, err := model.LoadStack(name, stackPaths, true, fs)
	if err != nil {
		if name == "" {
			return nil, err
		}
		s = &model.Stack{Name: name}
	}
	s.Namespace = okteto.GetContext().Namespace
	return s, nil
}
