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
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/fatih/color"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type SelectItem struct {
	Name   string
	Enable bool
}

func getKubernetesContextList(filterOkteto bool) []string {
	contextList := make([]string, 0)
	kubeconfigFile := config.GetKubeconfigPath()
	cfg := kubeconfig.Get(kubeconfigFile)
	if cfg == nil {
		return contextList
	}
	if !filterOkteto {
		return contextList
	}
	for name := range cfg.Contexts {
		if _, ok := cfg.Contexts[name].Extensions[model.OktetoExtension]; ok {
			continue
		}
		contextList = append(contextList, name)
	}
	return contextList
}

func isCreateNewContextOption(option string) bool {
	if option == newOEOption {
		return true
	}
	return okteto.IsOktetoURL(option)
}

func getOktetoClusterUrl(option string) string {

	if okteto.IsOktetoURL(option) {
		return option
	}

	return askForOktetoURL()
}

func askForOktetoURL() string {
	clusterURL := okteto.CloudURL
	ctxStore := okteto.ContextStore()
	if okteto.IsOktetoURL(ctxStore.CurrentContext) {
		clusterURL = ctxStore.CurrentContext
	}
	fmt.Print(color.MagentaString("What is the URL of your Okteto Cluster? [%s]: ", strings.TrimSuffix(clusterURL, "/")))
	fmt.Scanln(&clusterURL)

	url, err := url.Parse(clusterURL)
	if err != nil {
		return ""
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}
	return strings.TrimSuffix(url.String(), "/")
}

func isValidCluster(cluster string) bool {
	for _, c := range getKubernetesContextList(false) {
		if cluster == c {
			return true
		}
	}
	return false
}

func addKubernetesContext(cfg *clientcmdapi.Config, ctxResource *model.ContextResource) error {
	if cfg == nil {
		return fmt.Errorf(errors.ErrKubernetesContextNotFound, ctxResource.Context, config.GetKubeconfigPath())
	}
	if _, ok := cfg.Contexts[ctxResource.Context]; !ok {
		return fmt.Errorf(errors.ErrKubernetesContextNotFound, ctxResource.Context, config.GetKubeconfigPath())
	}
	if ctxResource.Namespace == "" {
		ctxResource.Namespace = cfg.Contexts[ctxResource.Context].Namespace
	}
	if ctxResource.Namespace == "" {
		ctxResource.Namespace = "default"
	}
	okteto.AddKubernetesContext(ctxResource.Context, ctxResource.Namespace, "")
	return nil
}

func LoadDevWithContext(ctx context.Context, devPath, namespace, k8sContext string) (*model.Dev, error) {
	ctxResource, err := utils.LoadDevContext(devPath)
	if err != nil {
		return nil, err
	}

	if err := ctxResource.UpdateNamespace(namespace); err != nil {
		return nil, err
	}

	if err := ctxResource.UpdateContext(k8sContext); err != nil {
		return nil, err
	}

	ctxOptions := &ContextOptions{
		Context:   ctxResource.Context,
		Namespace: ctxResource.Namespace,
		Show:      true,
	}
	if err := Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	return utils.LoadDev(devPath)
}

func LoadStackWithContext(ctx context.Context, name, namespace string, stackPaths []string) (*model.Stack, error) {
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

	ctxOptions := &ContextOptions{
		Context:   ctxResource.Context,
		Namespace: ctxResource.Namespace,
		Show:      true,
	}
	if err := Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	s, err := utils.LoadStack(name, stackPaths)
	if err != nil {
		if name == "" {
			return nil, err
		}
		s = &model.Stack{Name: name}
	}
	s.Namespace = okteto.Context().Namespace
	return s, nil
}
