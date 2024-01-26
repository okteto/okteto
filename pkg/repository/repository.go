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
	"context"
	"net/url"
	"os"
	"strings"

	giturls "github.com/chainguard-dev/git-urls"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// Repository is the struct to check everything related to Git Repo
// like checking the commit or if the project has changes over it
type Repository struct {
	control repositoryInterface

	url  *repositoryURL
	path string
}

type repositoryInterface interface {
	isClean(ctx context.Context) (bool, error)
	getSHA() (string, error)
	GetLatestDirCommit(string) (string, error)
	GetDiffHash(string) (string, error)
}

type repositoryURL struct {
	url.URL
}

// String is a custom implementation for the url where User is removed
func (r repositoryURL) String() string {
	repo := r.URL
	repo.User = nil
	return repo.String()
}

func getURLFromPath(path string) repositoryURL {
	url, err := giturls.Parse(path)
	if err != nil {
		oktetoLog.Infof("could not parse url: %w", err)
	}

	return repositoryURL{
		*url,
	}
}

// NewRepository creates a repository controller
func NewRepository(path string) Repository {
	repoURL := getURLFromPath(path)

	var controller repositoryInterface = newGitRepoController(path)
	// check if we are inside a remote deploy
	if v := os.Getenv(constants.OktetoDeployRemote); v != "" {
		sha := os.Getenv(constants.OktetoGitCommitEnvVar)
		controller = newOktetoRemoteRepoController(sha)
	}
	return Repository{
		path:    path,
		url:     &repoURL,
		control: controller,
	}
}

// IsClean checks if the repository have changes over the commit
func (r Repository) IsClean() (bool, error) {
	return r.control.isClean(context.TODO())
}

// GetSHA returns the last commit sha of the repository
func (r Repository) GetSHA() (string, error) {
	return r.control.getSHA()
}

// IsEqual checks if another repository is the same from the one calling the function
func (r Repository) IsEqual(otherRepo Repository) bool {
	if r.url == nil || otherRepo.url == nil {
		return false
	}

	if r.url.Hostname() != otherRepo.url.Hostname() {
		return false
	}

	// In short SSH URLs like git@github.com:okteto/movies.git, path doesn't start with '/', so we need to remove it
	// in case it exists. It also happens with '.git' suffix. You don't have to specify it, so we remove in both cases
	repoPathA := cleanPath(r.url.Path)
	repoPathB := cleanPath(otherRepo.url.Path)

	return repoPathA == repoPathB
}

func cleanPath(path string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".git")
}

// GetAnonymizedRepo returns a clean repo url string without sensible information
func (r Repository) GetAnonymizedRepo() string {
	return r.url.String()
}

func (r Repository) GetLatestDirCommit(dir string) (string, error) {
	return r.control.GetLatestDirCommit(dir)
}

func (r Repository) GetDiffHash(dir string) (string, error) {
	return r.control.GetDiffHash(dir)
}
