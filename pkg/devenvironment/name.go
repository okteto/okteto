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

package devenvironment

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/discovery"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/utils"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/spf13/afero"
	"k8s.io/client-go/kubernetes"
)

// DeprecatedInferName infers the dev environment name from the folder received as parameter.
// It is deprecated as it doesn't take into account deployed dev environments to get the non-sanitized name.
// This is only being effectively used in push command, which will be deleted in the next major version
func DeprecatedInferName(cwd string) string {
	repoURL, err := utils.GetRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	oktetoLog.Info("inferring name from git repository URL")
	return utils.TranslateURLToName(repoURL)
}

// NameInferer Allows to infer the name for a dev environment
type NameInferer struct {
	k8s              kubernetes.Interface
	getRepositoryURL func(string) (string, error)
	fs               afero.Fs
}

// NewNameInferer allows to create a new instance of a name inferer
func NewNameInferer(k8s kubernetes.Interface) NameInferer {
	return NameInferer{
		k8s:              k8s,
		getRepositoryURL: utils.GetRepositoryURL,
		fs:               afero.NewOsFs(),
	}
}

// InferNameFromDevEnvsAndRepository it infers the name from the development environments deployed in the specified namespace
// or from the git repository URL if no dev environment is found.
// `manifestPath` is needed because we compare it with the one in dev environments to see if it is the dev environment we look for
func (n NameInferer) InferNameFromDevEnvsAndRepository(ctx context.Context, repoURL, namespace, manifestPath, discoveredFile string) string {
	labelSelector := fmt.Sprintf("%s=true", model.GitDeployLabel)

	oktetoLog.Infof("found repository url %s", repoURL)
	cfList, err := configmaps.List(ctx, namespace, labelSelector, n.k8s)
	if err != nil {
		oktetoLog.Info("could not get deployed dev environments: %v. Inferring dev environment name from the repository URL", err)
		return utils.TranslateURLToName(repoURL)
	}

	oktetoLog.Infof("found '%d' configmaps in the namespace %s", len(cfList), namespace)

	// There might be several dev environments with the specified repository and manifest. We retrieve all possibilities
	var possibleNames []string
	for _, cmap := range cfList {
		oktetoLog.Infof("checking configmap %s", cmap.Name)
		repo := cmap.Data["repository"]
		if repo == "" {
			oktetoLog.Infof("configmap %s doesn't have a repository", cmap.Name)
			continue
		}

		optsRepo := repository.NewRepository(repoURL)
		cmapRepo := repository.NewRepository(repo)
		if !optsRepo.IsEqual(cmapRepo) {
			oktetoLog.Infof("configmap %s with repo %s doesn't match with found repo %s", cmap.Name, cmapRepo.GetAnonymizedRepo(), repoURL)
			continue
		}

		cmapFilename := cmap.Data["filename"]
		// If the manifestPath is not the default one, we compare it with the one in the configmap
		if manifestPath != cmapFilename {
			if manifestPath == "" {
				// If manifestPath is empty and filename is not equal to one of the default okteto name,
				// log a message indicating the mismatch between the configmap and the provided manifest.
				if cmapFilename != discoveredFile {
					oktetoLog.Infof("configmap %s with manifest %s doesn't match with provided manifest %s", cmapFilename, cmapRepo.GetAnonymizedRepo(), manifestPath)
					continue
				}
			} else {
				// If manifestPath is not empty and filename is not the same as manifestPath,
				// log a message indicating the mismatch between the configmap and the provided manifest.
				oktetoLog.Infof("configmap %s with manifest %s doesn't match with provided manifest %s", cmapFilename, cmapRepo.GetAnonymizedRepo(), manifestPath)
				continue
			}
		}
		possibleNames = append(possibleNames, cmap.Data["name"])
	}

	// if no names were found we infer the name from the repository URL
	if len(possibleNames) == 0 {
		oktetoLog.Info("inferring name from git repository URL")
		return utils.TranslateURLToName(repoURL)
	}

	// If more than 1 name is found, we print a message to the user know the name that was inferred
	if len(possibleNames) > 1 {
		oktetoLog.Warning("found several dev environments candidates to infer the name: %s. Using '%s'", strings.Join(possibleNames, ", "), possibleNames[0])
	}

	oktetoLog.Infof("inferred name from dev environment '%s'", possibleNames[0])
	return possibleNames[0]
}

// InferName infers the dev environment name from the folder received as parameter. It has the following preference:
//   - If cwd (current working directory) contains a repo, we look for a dev environment deployed with the same repository and the same
//     manifest path, and we took the name from the config map
//   - If not dev environment is found, we use the repository name to infer the dev environment name
//   - If the current working directory doesn't have a repository, we get the name from the folder name
//
// `manifestPath` is needed because we compare it with the one in dev environments to see if it is the dev environment we look for
func (n NameInferer) InferName(ctx context.Context, cwd, namespace, manifestPath string) string {
	repoURL, err := n.getRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	discoveredFile, err := discovery.GetOktetoManifestPathWithFilesystem(cwd, n.fs)
	if err != nil {
		oktetoLog.Info("could not detect okteto manifest file")
	}
	discoveredFile, err = filepath.Rel(cwd, discoveredFile)
	if err != nil {
		oktetoLog.Infof("could not get relative path for %s: %s", discoveredFile, err.Error())
	}
	// We need to sanitize paths to be UNIX style, as the ones in the configmaps are
	sanitizedFile := filepath.ToSlash(discoveredFile)

	return n.InferNameFromDevEnvsAndRepository(ctx, repoURL, namespace, manifestPath, sanitizedFile)
}
