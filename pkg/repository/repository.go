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
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	giturls "github.com/whilp/git-urls"
)

// Repository is the struct to check everything related to Git Repo
// like checking the commit or if the project has changes over it
type Repository struct {
	path string
	url  *repositoryURL

	repoControl controlRepositoryInterface
}

type RepositoryService struct {
	serviceControl controlRepositoryServiceInterface
}

type controlRepositoryInterface interface {
	isClean(ctx context.Context) (bool, error)
	getSHA() (string, error)
}

type controlRepositoryServiceInterface interface {
	isCleanDir(ctx context.Context, dir string) (bool, error)
	getHashByDir(string) (string, error)
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

	var controller controlRepositoryInterface = newGitRepoController()

	// check if we are inside a remote deploy
	if v := os.Getenv(constants.OktetoDeployRemote); v != "" {
		sha := os.Getenv(constants.OktetoGitCommitEnvVar)
		controller = newOktetoRemoteRepoController(sha)
	}

	return Repository{
		path:        path,
		url:         &repoURL,
		repoControl: controller,
	}
}

func NewRepositoryService(service string) RepositoryService {
	var serviceController controlRepositoryServiceInterface = newGitRepoController()

	if v := os.Getenv(constants.OktetoDeployRemote); v != "" {
		hash := os.Getenv(fmt.Sprintf(constants.OktetoServiceBuildHash, service))
		isClean, err := strconv.ParseBool(os.Getenv(fmt.Sprintf(constants.OktetoServiceIsClean, service)))
		if err != nil {
			oktetoLog.Infof("failed to check if service '%s' build context is clean from envs: %w", service, err)
		}
		serviceController = newOktetoRemoteServiceController(hash, isClean)
	}

	return RepositoryService{
		serviceControl: serviceController,
	}
}

// IsClean checks if the repository have changes over the commit
func (r Repository) IsClean() (bool, error) {
	return r.repoControl.isClean(context.TODO())
}

// IsBuildContextClean checks if the build context have changes over the context
func (r RepositoryService) IsBuildContextClean(buildContext string) (bool, error) {
	return r.serviceControl.isCleanDir(context.TODO(), buildContext)
}

// GetSHA returns the last commit sha of the repository
func (r Repository) GetSHA() (string, error) {
	return r.repoControl.getSHA()
}

// GetBuildHash returns the hash related to a build context
func (r RepositoryService) GetBuildContextHash(buildContext string) (string, error) {
	return r.serviceControl.getHashByDir(buildContext)
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
