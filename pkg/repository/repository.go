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

package repository

import (
	"net/url"
	"os"

	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	giturls "github.com/whilp/git-urls"
)

// Repository is the struct to check everything related to Git Repo
// like checking the commit or if the project has changes over it
type Repository struct {
	path string
	url  *url.URL

	control repositoryInterface
}

type repositoryInterface interface {
	isClean() (bool, error)
	getSHA() (string, error)
}

// NewRepository creates a repository controller
func NewRepository(path string) Repository {
	url, err := giturls.Parse(path)
	if err != nil {
		oktetoLog.Infof("could not parse url: %w", err)
	}

	var controller repositoryInterface = newGitRepoController()
	// check if we are inside a remote deploy
	if v := os.Getenv(constants.OKtetoDeployRemote); v != "" {
		sha := os.Getenv(constants.OktetoGitCommitEnvVar)
		controller = newOktetoRemoteRepoController(sha)
	}
	return Repository{
		path:    path,
		url:     url,
		control: controller,
	}
}

// IsClean checks if the repository have changes over the commit
func (r Repository) IsClean() (bool, error) {
	return r.control.isClean()
}

// GetSHA returns the last commit sha of the repository
func (r Repository) GetSHA() (string, error) {
	return r.control.getSHA()
}
