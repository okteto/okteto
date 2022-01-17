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
	"os"
	"strconv"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/constants"
	ctxModel "github.com/okteto/okteto/pkg/model/context"
	"github.com/okteto/okteto/pkg/model/manifest"
	"github.com/okteto/okteto/pkg/model/stack"
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

func askForOktetoURL() string {
	clusterURL := okteto.CloudURL
	ctxStore := okteto.ContextStore()
	if oCtx, ok := ctxStore.Contexts[ctxStore.CurrentContext]; ok && oCtx.IsOkteto {
		clusterURL = ctxStore.CurrentContext
	}

	log.Question("Enter your Okteto URL [%s]: ", clusterURL)
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

func addKubernetesContext(cfg *clientcmdapi.Config, ctxResource *ctxModel.ContextResource) error {
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

func LoadManifestWithContext(ctx context.Context, devPath, namespace, k8sContext string) (*manifest.Manifest, error) {
	ctxResource, err := utils.LoadManifestContext(devPath)
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

	if err := NewContextCommand().Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	return utils.LoadManifest(devPath)
}

func LoadStackWithContext(ctx context.Context, name, namespace string, stackPaths []string) (*stack.Stack, error) {
	ctxResource, err := utils.LoadStackContext(stackPaths)
	if err != nil {
		if name == "" {
			return nil, err
		}
		ctxResource = &ctxModel.ContextResource{}
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

	s, err := utils.LoadStack(name, stackPaths)
	if err != nil {
		if name == "" {
			return nil, err
		}
		s = &stack.Stack{Name: name}
	}
	s.Namespace = okteto.Context().Namespace
	return s, nil
}

//LoadManifestV2WithContext initializes the okteto context taking into account command flags and manifest namespace/context fields
func LoadManifestV2WithContext(ctx context.Context, namespace, path string) error {
	ctxOptions := &ContextOptions{
		Namespace: namespace,
		Show:      true,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	manifest, err := manifest.GetManifestV2(cwd)
	if err != nil {
		//GetManifestV2 should take care of all error conditions and possible paths
		//https://github.com/okteto/okteto/issues/2111
		if err != errors.ErrManifestNotFound {
			return err
		}
	} else {
		ctxOptions.Context = manifest.Context
		if ctxOptions.Namespace == "" {
			ctxOptions.Namespace = manifest.Namespace
		}
	}

	return NewContextCommand().Run(ctx, ctxOptions)
}

func IsManifestV2Enabled() bool {
	r, err := strconv.ParseBool(os.Getenv("OKTETO_ENABLE_MANIFEST_V2"))
	if err != nil {
		return false
	}
	return r
}
