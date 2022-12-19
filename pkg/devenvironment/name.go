package devenvironment

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

// DeprecatedInferName infers the dev environment name from the folder received as parameter.
// It is deprecated as it doesn't take into account deployed dev environments to get the non-sanitized name.
// This is only being effectively used in push command, which will be deleted in the mext major version
func DeprecatedInferName(cwd string) string {
	repoURL, err := model.GetRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	oktetoLog.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repoURL)
}

// InferName infers the dev environment name from the folder received as parameter. It has the following preference:
//   - If cwd (current working directory) contains a repo, we look for a dev environment deployed with the same repository and the same
//   manifest path, and we took the name from the config map
//   - If not dev environment is found, we use the repository name to infer the dev environment name
//   - If the current working directory doesn't have a repository, we get the name from the folder name
func InferName(ctx context.Context, cwd, namespace, manifestPath string, c kubernetes.Interface) string {
	repoURL, err := model.GetRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	labelSelector := fmt.Sprintf("%s=true", model.GitDeployLabel)

	oktetoLog.Information("found repository url %s", repoURL)
	cfList, err := configmaps.List(ctx, namespace, labelSelector, c)
	if err != nil {
		oktetoLog.Info("could not get deployed dev environments: %v. Inferring dev environment name from the repository URL", err)
		return model.TranslateURLToName(repoURL)
	}

	oktetoLog.Information("found %d configmaps in the namespace %s", len(cfList), namespace)

	for _, cmap := range cfList {
		oktetoLog.Information("checking configmap %s", cmap.Name)
		repo := cmap.Data["repository"]
		if repo == "" {
			oktetoLog.Information("configmap %s doesn't have a repository", cmap.Name)
			continue
		}

		if !okteto.AreSameRepository(repoURL, repo) {
			oktetoLog.Information("configmap %s with repo %s doesn't match with found repo %s", cmap.Name, repo, repoURL)
			continue
		}

		if filename := cmap.Data["filename"]; filename != manifestPath {
			oktetoLog.Information("configmap %s with manifest %s doesn't match with provided manifest %s", filename, repo, manifestPath)
			continue
		}

		return cmap.Data["name"]
	}

	oktetoLog.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repoURL)
}
