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

// InferName infers the application name from the folder received as parameter
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
			oktetoLog.Information("configmap %s doesn't have a repo", cmap.Name)
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
