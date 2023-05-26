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
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"strings"
	"time"

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

type cleanStatus struct {
	IsClean bool
	Err     error
}

// IsClean checks if the repository have changes over the commit
func (r gitRepoController) isClean(ctx context.Context) (bool, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	ch := make(chan cleanStatus)
	defer close(ch)

	go func() {
		repo, err := r.repoGetter.get(r.path)
		if err != nil {
			ch <- cleanStatus{false, fmt.Errorf("failed to analyze git repo: %w", err)}
			return
		}

		worktree, err := repo.Worktree()
		if err != nil {
			ch <- cleanStatus{false, fmt.Errorf("failed to infer the git repo's current branch: %w", err)}
			return
		}

		status, err := worktree.Status(ctx, NewLocalGit("git", &LocalExec{}))
		if err != nil {
			ch <- cleanStatus{false, fmt.Errorf("failed to infer the git repo's status: %w", err)}
			return
		}

		ch <- cleanStatus{status.IsClean(), nil}
	}()

	select {
	case <-ctxWithTimeout.Done():
		oktetoLog.Warning("Timeout exceeded calculating git status: assuming dirty commit")
		return false, ctxWithTimeout.Err()
	case res := <-ch:
		return res.IsClean, res.Err

	}
}

// GetSHA returns the last commit sha of the repository
func (r gitRepoController) getSHA() (string, error) {
	isClean, err := r.isClean(context.TODO())
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

func (ogr oktetoGitWorktree) GetRoot() string {
	return ogr.worktree.Filesystem.Root()
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
	Status(context.Context, LocalGitInterface) (oktetoGitStatus, error)
	GetRoot() string
}

func (ogr oktetoGitWorktree) Status(ctx context.Context, localGit LocalGitInterface) (oktetoGitStatus, error) {
	// using git directly is faster, so we check if it's available
	_, err := localGit.Exists()
	if err != nil {
		// git is not available, so we fall back on git-go
		oktetoLog.Warning("Calculating git status: git is not installed, for better performances consider installing it")
		status, err := ogr.worktree.Status()
		if err != nil {
			return oktetoGitStatus{status: git.Status{}}, fmt.Errorf("failed to get git status: %w", err)
		}

		return oktetoGitStatus{status: status}, nil
	}

	output, err := localGit.Status(ctx, ogr.GetRoot(), 0)
	if err != nil {
		return oktetoGitStatus{status: git.Status{}}, fmt.Errorf("failed to get git status: %w", err)
	}

	lines := strings.Split(output, "\000")
	stat := make(map[string]*git.FileStatus, len(lines))
	for _, line := range lines {
		// line example values can be: "M modified-file.go", "?? new-file.go", etc
		parts := strings.SplitN(strings.TrimLeft(line, " "), " ", 2)
		if len(parts) == 2 {
			stat[strings.Trim(parts[1], " ")] = &git.FileStatus{
				Staging: git.StatusCode([]byte(parts[0])[0]),
			}
		} else {
			return oktetoGitStatus{status: git.Status{}}, fmt.Errorf("failed to get git status: unexpected status line")
		}
	}

	return oktetoGitStatus{status: stat}, nil
}
