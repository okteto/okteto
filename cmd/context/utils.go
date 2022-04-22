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

package context

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
		if _, ok := cfg.Contexts[name].Extensions[model.OktetoExtension]; ok && filterOkteto {
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

func askForOktetoURL() (string, error) {
	clusterURL := okteto.CloudURL
	ctxStore := okteto.ContextStore()
	if oCtx, ok := ctxStore.Contexts[ctxStore.CurrentContext]; ok && oCtx.IsOkteto {
		clusterURL = ctxStore.CurrentContext
	}

	err := oktetoLog.Question("Enter your Okteto URL [%s]: ", clusterURL)
	if err != nil {
		return "", err
	}
	fmt.Scanln(&clusterURL)

	url, err := url.Parse(clusterURL)
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

func addKubernetesContext(cfg *clientcmdapi.Config, ctxResource *model.ContextResource) error {
	if cfg == nil {
		return fmt.Errorf(oktetoErrors.ErrKubernetesContextNotFound, ctxResource.Context, config.GetKubeconfigPath())
	}
	if _, ok := cfg.Contexts[ctxResource.Context]; !ok {
		return fmt.Errorf(oktetoErrors.ErrKubernetesContextNotFound, ctxResource.Context, config.GetKubeconfigPath())
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

func LoadManifestWithContext(ctx context.Context, opts ManifestOptions) (*model.Manifest, error) {
	m, err := model.GetManifestV2(opts.Filename)
	if err != nil {
		return nil, err
	}

	ctxResource := model.GetContextResourceFromManifest(m)
	if err := ctxResource.UpdateNamespace(opts.Namespace); err != nil {
		return nil, err
	}

	if err := ctxResource.UpdateContext(opts.K8sContext); err != nil {
		return nil, err
	}

	ctxOptions := &ContextOptions{
		Context:       ctxResource.Context,
		Namespace:     ctxResource.Namespace,
		Show:          true,
		DockerDesktop: os.Getenv(model.OktetoOriginEnvVar) == model.OktetoDockerDesktopOrigin,
	}

	if err := NewContextCommand().Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	// We need to read it again to propagate secrets env vars
	m, err = model.GetManifestV2(opts.Filename)
	if err != nil {
		return nil, err
	}
	m.Namespace = okteto.Context().Namespace
	m.Context = okteto.Context().Name

	for _, dev := range m.Dev {
		if err := utils.LoadManifestRc(dev); err != nil {
			return nil, err
		}

		dev.Namespace = okteto.Context().Namespace
		dev.Context = okteto.Context().Name
	}

	return m, nil
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

	if err := NewContextCommand().Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	s, err := model.LoadStack(name, stackPaths, true)
	if err != nil {
		if name == "" {
			return nil, err
		}
		s = &model.Stack{Name: name}
	}
	s.Namespace = okteto.Context().Namespace
	return s, nil
}

//LoadManifestV2WithContext initializes the okteto context taking into account command flags and manifest namespace/context fields
func LoadManifestV2WithContext(ctx context.Context, namespace, k8sContext, path string) error {
	ctxOptions := &ContextOptions{
		Namespace: namespace,
		Show:      true,
	}

	manifest, err := model.GetManifestV2(path)
	if err != nil {
		if err != oktetoErrors.ErrManifestNotFound {
			return err
		}
	} else {
		ctxResource := model.GetContextResourceFromManifest(manifest)

		if err := ctxResource.UpdateNamespace(namespace); err != nil {
			return err
		}

		if err := ctxResource.UpdateContext(k8sContext); err != nil {
			return err
		}

		ctxOptions = &ContextOptions{
			Context:   ctxResource.Context,
			Namespace: ctxResource.Namespace,
			Show:      true,
		}
	}

	return NewContextCommand().Run(ctx, ctxOptions)
}
