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
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	errNotCleanRepo = errors.New("repository is not clean")
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
	isClean bool
	err     error
}

func (r gitRepoController) calculateIsClean(ctx context.Context) (bool, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return false, fmt.Errorf("failed to analyze git repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's current branch: %w", err)
	}

	status, err := worktree.Status(ctx, NewLocalGit("git", &LocalExec{}))
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's status: %w", err)
	}

	return status.IsClean(), nil
}

// isClean checks if the repository have changes over the commit
func (r gitRepoController) isClean(ctx context.Context) (bool, error) {
	// We use context.TODO() in a few places to call isClean, so let's make sure
	// we set proper internal timeouts to not leak goroutines
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	timeoutCh := make(chan struct{})
	ch := make(chan cleanStatus)

	timeoutErr := errors.New("timeout exceeded")

	go func() {
		time.Sleep(time.Second)
		close(timeoutCh)
		cancel()
		ch <- cleanStatus{false, timeoutErr}
	}()

	go func() {
		clean, err := r.calculateIsClean(ctx)
		select {
		case <-timeoutCh:
		case ch <- cleanStatus{clean, err}:
		}
	}()

	s := <-ch

	if s.err == timeoutErr {
		oktetoLog.Debug("Timeout exceeded calculating git status: assuming dirty commit")
	}

	return s.isClean, s.err
}

// GetSHA returns the last commit sha of the repository
func (r gitRepoController) getSHA() (string, error) {
	isClean, err := r.isClean(context.TODO())
	if err != nil {
		return "", fmt.Errorf("%w: failed to check if repo is clean: %w", errNotCleanRepo, err)
	}
	if !isClean {
		return "", errNotCleanRepo
	}
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("%w: failed to analyze git repo: %w", errNotCleanRepo, err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("%w: failed to analyze git repo: %w", errNotCleanRepo, err)
	}
	return head.Hash().String(), nil
}

func (r gitRepoController) getTreeSHA(context string) (string, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD from repo: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit object from reference: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get tree from commit: %w", err)
	}

	if context == "." {
		return tree.Hash.String(), nil
	}

	treeDir, err := getTreeDirFromRelPath(context)
	if err != nil {
		oktetoLog.Infof("could not tree working dir: %w", err)
		return "", err
	}

	svcEntry, err := tree.FindEntry(treeDir)
	if err != nil {
		return "", fmt.Errorf("failed to find an entry in tree: %w", err)
	}

	return svcEntry.Hash.String(), nil
}

func getTreeDirFromRelPath(relPath string) (string, error) {

	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		return "", err
	}

	return filepath.Base(filepath.Join(wd, relPath)), nil
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

func (ogr oktetoGitRepository) CommitObject(h plumbing.Hash) (gitCommitInterface, error) {
	return ogr.repo.CommitObject(h)
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
	CommitObject(plumbing.Hash) (gitCommitInterface, error)
}

type gitCommitInterface interface {
	Tree() (*object.Tree, error)
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
		oktetoLog.Debug("Calculating git status: git is not installed, for better performances consider installing it")
		status, err := ogr.worktree.Status()
		if err != nil {
			return oktetoGitStatus{status: git.Status{}}, fmt.Errorf("failed to get git status: %w", err)
		}

		return oktetoGitStatus{status: status}, nil
	}

	status, err := localGit.Status(ctx, ogr.GetRoot(), 0)
	if err != nil {
		return oktetoGitStatus{status: git.Status{}}, fmt.Errorf("failed to get git status: %w", err)
	}

	return oktetoGitStatus{status: status}, nil
}
