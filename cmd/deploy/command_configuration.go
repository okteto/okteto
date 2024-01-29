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

package deploy

import (
	"context"
	"net/url"
	"os"
	"reflect"

	giturls "github.com/chainguard-dev/git-urls"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	ioCtrl "github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	modelUtils "github.com/okteto/okteto/pkg/model/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/repository"
	"k8s.io/client-go/kubernetes"
)

const (
	sshScheme   = "ssh"
	httpScheme  = "http"
	httpsScheme = "https"
)

func setDeployOptionsValuesFromManifest(ctx context.Context, deployOptions *Options, cwd string, c kubernetes.Interface, k8sLogger *ioCtrl.K8sLogger) error {

	if deployOptions.Manifest.Context == "" {
		deployOptions.Manifest.Context = okteto.GetContext().Name
	}
	if deployOptions.Manifest.Namespace == "" {
		deployOptions.Manifest.Namespace = okteto.GetContext().Namespace
	}

	if deployOptions.Name == "" {
		if deployOptions.Manifest.Name != "" {
			deployOptions.Name = deployOptions.Manifest.Name
		} else {
			c, _, err := okteto.NewK8sClientProviderWithLogger(k8sLogger).Provide(okteto.GetContext().Cfg)
			if err != nil {
				return err
			}
			inferer := devenvironment.NewNameInferer(c)
			deployOptions.Name = inferer.InferName(ctx, cwd, okteto.GetContext().Namespace, deployOptions.ManifestPathFlag)
			deployOptions.Manifest.Name = deployOptions.Name
		}

	} else {
		if deployOptions.Manifest != nil {
			deployOptions.Manifest.Name = deployOptions.Name
		}
		if deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.ComposeSection != nil && deployOptions.Manifest.Deploy.ComposeSection.Stack != nil {
			// when deploy options has name, stack name is overridden
			// this name might not be sanitized
			deployOptions.Manifest.Deploy.ComposeSection.Stack.Name = deployOptions.Name
		}
	}

	if deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.ComposeSection != nil && deployOptions.Manifest.Deploy.ComposeSection.Stack != nil {

		mergeServicesToDeployFromOptionsAndManifest(deployOptions)
		if len(deployOptions.servicesToDeploy) == 0 {
			deployOptions.servicesToDeploy = []string{}
			for service := range deployOptions.Manifest.Deploy.ComposeSection.Stack.Services {
				deployOptions.servicesToDeploy = append(deployOptions.servicesToDeploy, service)
			}
		}
		if len(deployOptions.Manifest.Deploy.ComposeSection.ComposesInfo) > 0 {
			if err := stack.ValidateDefinedServices(deployOptions.Manifest.Deploy.ComposeSection.Stack, deployOptions.servicesToDeploy); err != nil {
				return err
			}
			deployOptions.servicesToDeploy = stack.AddDependentServicesIfNotPresent(ctx, deployOptions.Manifest.Deploy.ComposeSection.Stack, deployOptions.servicesToDeploy, c)
			deployOptions.Manifest.Deploy.ComposeSection.ComposesInfo[0].ServicesToDeploy = deployOptions.servicesToDeploy
		}
	}
	return nil
}

func mergeServicesToDeployFromOptionsAndManifest(deployOptions *Options) {
	var manifestDeclaredServicesToDeploy []string
	for _, composeInfo := range deployOptions.Manifest.Deploy.ComposeSection.ComposesInfo {
		manifestDeclaredServicesToDeploy = append(manifestDeclaredServicesToDeploy, composeInfo.ServicesToDeploy...)
	}

	manifestDeclaredServicesToDeploySet := map[string]bool{}
	for _, service := range manifestDeclaredServicesToDeploy {
		manifestDeclaredServicesToDeploySet[service] = true
	}

	commandDeclaredServicesToDeploy := map[string]bool{}
	for _, service := range deployOptions.servicesToDeploy {
		commandDeclaredServicesToDeploy[service] = true
	}

	if reflect.DeepEqual(manifestDeclaredServicesToDeploySet, commandDeclaredServicesToDeploy) {
		return
	}

	if len(deployOptions.servicesToDeploy) > 0 && len(manifestDeclaredServicesToDeploy) > 0 {
		oktetoLog.Warning("overwriting manifest's `services to deploy` with command line arguments")
	}
	if len(deployOptions.servicesToDeploy) == 0 && len(manifestDeclaredServicesToDeploy) > 0 {
		deployOptions.servicesToDeploy = manifestDeclaredServicesToDeploy
	}
}

func (dc *Command) addEnvVars(cwd string) {
	if os.Getenv(constants.OktetoGitBranchEnvVar) == "" {
		branch, err := utils.GetBranch(cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve branch name: %s", err)
		}
		os.Setenv(constants.OktetoGitBranchEnvVar, branch)
	}

	if os.Getenv(model.GithubRepositoryEnvVar) == "" {
		repo, err := modelUtils.GetRepositoryURL(cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve repo name: %s", err)
		}

		if repo != "" {
			repoHTTPS := switchRepoSchemaToHTTPS(repo)
			if repoHTTPS == nil {
				// fallback to empty repository
				repo = ""
			} else {
				repo = repoHTTPS.String()
			}
		}
		os.Setenv(model.GithubRepositoryEnvVar, repo)
	}

	if os.Getenv(constants.OktetoGitCommitEnvVar) == "" {
		sha, err := repository.NewRepository(cwd).GetSHA()
		if err != nil {
			oktetoLog.Infof("could not retrieve sha: %s", err)
		}
		isClean := true
		if !dc.isRemote {
			isClean, err = repository.NewRepository(cwd).IsClean()
			if err != nil {
				oktetoLog.Infof("could not status: %s", err)
			}
		}
		if !isClean {
			sha = utils.GetRandomSHA()
		}
		os.Setenv(constants.OktetoGitCommitEnvVar, sha)
	}
	if os.Getenv(model.OktetoRegistryURLEnvVar) == "" {
		os.Setenv(model.OktetoRegistryURLEnvVar, okteto.GetContext().Registry)
	}
	if os.Getenv(model.OktetoBuildkitHostURLEnvVar) == "" {
		os.Setenv(model.OktetoBuildkitHostURLEnvVar, okteto.GetContext().Builder)
	}
	if os.Getenv(model.OktetoTokenEnvVar) == "" {
		os.Setenv(model.OktetoTokenEnvVar, okteto.GetContext().Token)
	}
	oktetoLog.AddMaskedWord(os.Getenv(model.OktetoTokenEnvVar))
}

func switchRepoSchemaToHTTPS(repo string) *url.URL {
	repoURL, err := giturls.Parse(repo)
	if err != nil {
		return nil
	}
	switch repoURL.Scheme {
	case sshScheme, httpScheme:
		repoURL.Scheme = httpsScheme
		repoURL.User = nil
		return repoURL
	case httpsScheme:
		return repoURL
	default:
		// if repo was parsed but has not a valid schema
		oktetoLog.Infof("retrieved schema for %s - %s", repo, repoURL.Scheme)
		return nil
	}
}
