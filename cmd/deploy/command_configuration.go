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
	"context"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/stack"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	giturls "github.com/whilp/git-urls"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func setDeployOptionsValuesFromManifest(ctx context.Context, deployOptions *Options, manifest *model.Manifest, cwd string, c kubernetes.Interface) error {
	if manifest.Context == "" {
		manifest.Context = okteto.Context().Name
	}
	if manifest.Namespace == "" {
		manifest.Namespace = okteto.Context().Namespace
	}

	if deployOptions.Name == "" {
		if manifest.Name != "" {
			deployOptions.Name = manifest.Name
		} else {
			deployOptions.Name = utils.InferName(cwd)
			manifest.Name = deployOptions.Name
		}

	} else {
		if manifest != nil {
			manifest.Name = deployOptions.Name
		}
		if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
			manifest.Deploy.ComposeSection.Stack.Name = deployOptions.Name
		}
	}

	if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {

		deployOptions.servicesToDeploy = mergeServicesToDeployFromOptionsAndManifest(deployOptions.servicesToDeploy, manifest.Deploy.ComposeSection.ComposesInfo)
		if len(deployOptions.servicesToDeploy) == 0 {
			deployOptions.servicesToDeploy = []string{}
			for service := range manifest.Deploy.ComposeSection.Stack.Services {
				deployOptions.servicesToDeploy = append(deployOptions.servicesToDeploy, service)
			}
		}
		if len(manifest.Deploy.ComposeSection.ComposesInfo) > 0 {
			if err := stack.ValidateDefinedServices(manifest.Deploy.ComposeSection.Stack, deployOptions.servicesToDeploy); err != nil {
				return err
			}
			deployOptions.servicesToDeploy = stack.AddDependentServicesIfNotPresent(ctx, manifest.Deploy.ComposeSection.Stack, deployOptions.servicesToDeploy, c)
			manifest.Deploy.ComposeSection.ComposesInfo[0].ServicesToDeploy = deployOptions.servicesToDeploy
		}
	}
	return nil
}

func mergeServicesToDeployFromOptionsAndManifest(servicesToDeployFromOptions []string, composesInfo model.ComposeInfoList) []string {
	var manifestDeclaredServicesToDeploy []string
	for _, composeInfo := range composesInfo {
		manifestDeclaredServicesToDeploy = append(manifestDeclaredServicesToDeploy, composeInfo.ServicesToDeploy...)
	}

	manifestDeclaredServicesToDeploySet := map[string]bool{}
	for _, service := range manifestDeclaredServicesToDeploy {
		manifestDeclaredServicesToDeploySet[service] = true
	}

	commandDeclaredServicesToDeploy := map[string]bool{}
	for _, service := range servicesToDeployFromOptions {
		commandDeclaredServicesToDeploy[service] = true
	}

	if reflect.DeepEqual(manifestDeclaredServicesToDeploySet, commandDeclaredServicesToDeploy) {
		return servicesToDeployFromOptions
	}

	if len(servicesToDeployFromOptions) > 0 && len(manifestDeclaredServicesToDeploy) > 0 {
		oktetoLog.Warning("overwriting manifest's `services to deploy` with command line arguments")
	}
	if len(servicesToDeployFromOptions) == 0 && len(manifestDeclaredServicesToDeploy) > 0 {
		return manifestDeclaredServicesToDeploy
	}

	return servicesToDeployFromOptions
}

func addEnvVars(ctx context.Context, cwd string) error {
	if os.Getenv(model.OktetoGitBranchEnvVar) == "" {
		branch, err := utils.GetBranch(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve branch name: %s", err)
		}
		os.Setenv(model.OktetoGitBranchEnvVar, branch)
	}

	if os.Getenv(model.GithubRepositoryEnvVar) == "" {
		repo, err := model.GetRepositoryURL(cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve repo name: %s", err)
		}

		if repo != "" {
			repoHTTPS, err := switchSSHRepoToHTTPS(repo)
			if err != nil {
				return err
			}
			repo = repoHTTPS.String()
		}
		os.Setenv(model.GithubRepositoryEnvVar, repo)
	}

	if os.Getenv(model.OktetoGitCommitEnvVar) == "" {
		sha, err := utils.GetGitCommit(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve sha: %s", err)
		}
		isClean, err := utils.IsCleanDirectory(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not status: %s", err)
		}
		if !isClean {
			sha = utils.GetRandomSHA(ctx, cwd)
		}
		os.Setenv(model.OktetoGitCommitEnvVar, sha)
	}
	if os.Getenv(model.OktetoRegistryURLEnvVar) == "" {
		os.Setenv(model.OktetoRegistryURLEnvVar, okteto.Context().Registry)
	}
	if os.Getenv(model.OktetoBuildkitHostURLEnvVar) == "" {
		os.Setenv(model.OktetoBuildkitHostURLEnvVar, okteto.Context().Builder)
	}
	return nil
}

func switchSSHRepoToHTTPS(repo string) (*url.URL, error) {
	repoURL, err := giturls.Parse(repo)
	if err != nil {
		return nil, err
	}
	if repoURL.Scheme == "ssh" {
		repoURL.Scheme = "https"
		repoURL.User = nil
		repoURL.Path = strings.TrimSuffix(repoURL.Path, ".git")
		return repoURL, nil
	}
	if repoURL.Scheme == "https" {
		return repoURL, nil
	}

	return nil, fmt.Errorf("could not detect repo protocol")
}

func updateConfigMapStatusError(ctx context.Context, cfg *corev1.ConfigMap, c kubernetes.Interface, data *pipeline.CfgData, errMain error) error {
	if err := updateConfigMapStatus(ctx, cfg, c, data, errMain); err != nil {
		return err
	}

	return errMain
}

func getConfigMapFromData(ctx context.Context, data *pipeline.CfgData, c kubernetes.Interface) (*corev1.ConfigMap, error) {
	return pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
}

func updateConfigMapStatus(ctx context.Context, cfg *corev1.ConfigMap, c kubernetes.Interface, data *pipeline.CfgData, err error) error {
	oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, err.Error())
	data.Status = pipeline.ErrorStatus

	return pipeline.UpdateConfigMap(ctx, cfg, data, c)
}
