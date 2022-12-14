package devenvironment

import (
	"path/filepath"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

// InferName infers the application name from the folder received as parameter
func InferName(cwd string) string {
	repoURL, err := model.GetRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	oktetoLog.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repoURL)
}
