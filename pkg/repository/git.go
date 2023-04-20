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
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type gitRepoController struct {
	path       string
	repoGetter repositoryGetterInterface
}

func newGitRepoController() gitRepoController {
	return gitRepoController{
		repoGetter: gitRepositoryGetter{},
	}
}

// IsClean checks if the repository have changes over the commit
func (r gitRepoController) isClean() (bool, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return false, fmt.Errorf("failed to analyze git repo: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's current branch: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's status: %w", err)
	}

	return status.IsClean(), nil
}

// GetSHA returns the last commit sha of the repository
func (r gitRepoController) getSHA() (string, error) {
	isClean, err := r.isClean()
	if err != nil {
		return "", fmt.Errorf("failed to check if repo is clean: %w", err)
	}
	if !isClean {
		return "", nil
	}
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}
	return head.Hash().String(), nil
}

type repositoryGetterInterface interface {
	get(path string) (gitRepositoryInterface, error)
}

type gitRepositoryGetter struct{}

func (gitRepositoryGetter) get(path string) (gitRepositoryInterface, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return oktetoGitRepository{repo: repo}, nil
}

type oktetoGitRepository struct {
	repo *git.Repository
}

func (ogr oktetoGitRepository) Worktree() (gitWorktreeInterface, error) {
	worktree, err := ogr.repo.Worktree()
	if err != nil {
		return nil, err
	}
	return oktetoGitWorktree{worktree: worktree}, nil
}

func (ogr oktetoGitRepository) Head() (*plumbing.Reference, error) {
	return ogr.repo.Head()
}

type oktetoGitWorktree struct {
	worktree *git.Worktree
}

func (ogr oktetoGitWorktree) Status() (gitStatusInterface, error) {
	status, err := ogr.worktree.Status()
	if err != nil {
		return nil, err
	}
	return oktetoGitStatus{status: status}, nil
}

type oktetoGitStatus struct {
	status git.Status
}

func (ogs oktetoGitStatus) IsClean() bool {
	return ogs.status.IsClean()
}

type gitRepositoryInterface interface {
	Worktree() (gitWorktreeInterface, error)
	Head() (*plumbing.Reference, error)
}
type gitWorktreeInterface interface {
	Status() (gitStatusInterface, error)
}
type gitStatusInterface interface {
	IsClean() bool
}
