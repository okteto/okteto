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
	"path/filepath"
	"strconv"
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
	ctxResource, err := utils.LoadManifestContext(opts.Filename)
	if err != nil {
		return nil, err
	}

	if err := ctxResource.UpdateNamespace(opts.Namespace); err != nil {
		return nil, err
	}

	if err := ctxResource.UpdateContext(opts.K8sContext); err != nil {
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

	return utils.LoadManifest(opts.Filename)
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

	manifest, err := GetManifestV2(cwd, path)
	if err != nil {
		//GetManifestV2 should take care of all error conditions and possible paths
		//https://github.com/okteto/okteto/issues/2111
		if err != oktetoErrors.ErrManifestNotFound {
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

// GetManifest Loads a manifest
func GetManifest(srcFolder string, opts ManifestOptions) (*model.Manifest, error) {
	pipelinePath := getPipelinePath(srcFolder, opts.Filename)
	if pipelinePath != "" {
		oktetoLog.Debugf("Found okteto manifest %s", pipelinePath)
		manifest, err := utils.LoadManifest(pipelinePath)
		if err != nil {
			oktetoLog.Infof("could not load manifest: %s", err.Error())
			return nil, err
		}

		manifest.Type = "pipeline"
		manifest.Filename = pipelinePath
		return manifest, nil
	}

	src := srcFolder
	path := filepath.Join(srcFolder, opts.Filename)
	if opts.Filename != "" && pathExistsAndDir(path) {
		src = path
	}

	oktetoSubPath := getOktetoSubPath(srcFolder, src)
	chartSubPath := getChartsSubPath(srcFolder, src)
	if chartSubPath != "" {
		oktetoLog.Infof("Found chart")
		return &model.Manifest{
			Type:     "chart",
			Deploy:   &model.DeployInfo{Commands: []string{fmt.Sprintf("helm upgrade --install %s %s", opts.Name, chartSubPath)}},
			Filename: chartSubPath,
		}, nil
	}

	manifestsSubPath := getManifestsSubPath(srcFolder, src)
	if manifestsSubPath != "" {
		oktetoLog.Infof("Found kubernetes manifests")
		return &model.Manifest{
			Type:     "kubernetes",
			Deploy:   &model.DeployInfo{Commands: []string{fmt.Sprintf("kubectl apply -f %s", manifestsSubPath)}},
			Filename: manifestsSubPath,
		}, nil
	}

	stackSubPath := getStackSubPath(srcFolder, src)
	if stackSubPath != "" {
		oktetoLog.Infof("Found okteto stack")
		return &model.Manifest{
			Type:     "stack",
			Deploy:   &model.DeployInfo{Commands: []string{fmt.Sprintf("okteto stack deploy --build -f %s", stackSubPath)}},
			Filename: stackSubPath,
		}, nil
	}

	if oktetoSubPath != "" {
		oktetoLog.Infof("Found okteto manifest")
		return &model.Manifest{
			Type:     "okteto",
			Deploy:   &model.DeployInfo{Commands: []string{"okteto push --deploy"}},
			Filename: oktetoSubPath,
		}, nil
	}

	return nil, fmt.Errorf("file okteto manifest not found. See https://okteto.com/docs/cloud/okteto-pipeline for details on how to configure your git repository with okteto")
}

func getPipelinePath(src, filename string) string {
	path := filepath.Join(src, filename)
	if filename != "" && fileExistsAndNotDir(path) {
		return path
	}

	baseDir := src
	if filename != "" && pathExistsAndDir(path) {
		baseDir = path
	}

	// Files will be checked in the order defined in the list
	files := []string{
		"okteto-pipeline.yml",
		"okteto-pipeline.yaml",
		"okteto-pipelines.yml",
		"okteto-pipelines.yaml",
		".okteto/okteto-pipeline.yml",
		".okteto/okteto-pipeline.yaml",
		".okteto/okteto-pipelines.yml",
		".okteto/okteto-pipelines.yaml",
	}

	for _, name := range files {
		path := filepath.Join(baseDir, name)
		if fileExistsAndNotDir(path) {
			return path
		}
	}
	return ""
}

func getChartsSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	for _, name := range []string{"chart", "charts", "helm/chart", "helm/charts"} {
		path := filepath.Join(src, name, "Chart.yaml")
		if model.FileExists(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func getManifestsSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	for _, name := range []string{"manifests", "manifests.yml", "manifests.yaml", "kubernetes", "kubernetes.yml", "kubernetes.yaml", "k8s", "k8s.yml", "k8s.yaml"} {
		path := filepath.Join(src, name)
		if model.FileExists(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func getStackSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	files := []string{
		"okteto-stack.yml",
		"okteto-stack.yaml",
		"stack.yml",
		"stack.yaml",
		".okteto/okteto-stack.yml",
		".okteto/okteto-stack.yaml",
		".okteto/stack.yml",
		".okteto/stack.yaml",
		"docker-compose.yml",
		"docker-compose.yaml",
		".okteto/docker-compose.yml",
		".okteto/docker-compose.yaml",
	}
	for _, name := range files {
		path := filepath.Join(src, name)
		if fileExistsAndNotDir(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func getOktetoSubPath(basePath, src string) string {
	// Files will be checked in the order defined in the list
	files := []string{
		"okteto.yml",
		"okteto.yaml",
		".okteto/okteto.yml",
		".okteto/okteto.yaml",
	}
	for _, name := range files {
		path := filepath.Join(src, name)
		if fileExistsAndNotDir(path) {
			prefix := fmt.Sprintf("%s/", basePath)
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func pathExistsAndDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func fileExistsAndNotDir(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetOktetoManifestPath(file string) string {

	// Files will be checked in the order defined in the list
	files := []string{
		file,
		"okteto.yml",
		"okteto.yaml",
		".okteto/okteto.yml",
		".okteto/okteto.yaml",
	}
	for _, name := range files {
		if err := utils.CheckIfRegularFile(name); err == nil {
			return name
		}
	}
	return ""
}

func GetManifestV2(basePath, file string) (*model.Manifest, error) {
	manifestPath := ""
	if file != "" && fileExistsAndNotDir(file) {
		manifestPath = file
	} else {
		src := basePath
		manifestPath = getOktetoSubPath(basePath, src)
	}

	if manifestPath != "" {
		return model.Get(manifestPath)
	}
	return nil, oktetoErrors.ErrManifestNotFound
}

func IsManifestV2Enabled() bool {
	r, err := strconv.ParseBool(os.Getenv("OKTETO_ENABLE_MANIFEST_V2"))
	if err != nil {
		return false
	}
	return r
}
