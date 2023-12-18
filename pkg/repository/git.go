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

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

var (
	errNotCleanRepo    = errors.New("repository is not clean")
	errTimeoutExceeded = errors.New("timeout exceeded")
	errFindingRepo     = errors.New("top level git repo directory cannot be found")
)

type gitRepoController struct {
	repoGetter repositoryGetterInterface
	path       string
}

func newGitRepoController(path string) gitRepoController {
	return gitRepoController{
		repoGetter: gitRepositoryGetter{},
		path:       path,
	}
}

type cleanStatus struct {
	err     error
	isClean bool
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

	go func() {
		time.Sleep(time.Second)
		close(timeoutCh)
		cancel()
		ch <- cleanStatus{errTimeoutExceeded, false}
	}()

	go func() {
		clean, err := r.calculateIsClean(ctx)
		select {
		case <-timeoutCh:
		case ch <- cleanStatus{err, clean}:
		}
	}()

	s := <-ch

	if errors.Is(s.err, errTimeoutExceeded) {
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

type commitResponse struct {
	err    error
	commit string
}

func (r gitRepoController) GeLatestDirCommit(contextDir string) (string, error) {
	// We use context.TODO() in a few places to call isClean, so let's make sure
	// we set proper internal timeouts to not leak goroutines
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	timeoutCh := make(chan struct{})
	ch := make(chan commitResponse)

	timeout := 1 * time.Second

	go func() {
		time.Sleep(timeout)
		close(timeoutCh)
		cancel()
		ch <- commitResponse{
			commit: "",
			err:    errTimeoutExceeded,
		}
	}()

	go func() {
		commit, err := r.calculateLatestDirCommit(ctx, contextDir)
		select {
		case <-timeoutCh:
		case ch <- commitResponse{
			commit: commit,
			err:    err,
		}:
		}
	}()

	s := <-ch

	if errors.Is(s.err, errTimeoutExceeded) {
		oktetoLog.Debugf("Timeout exceeded calculating git commit for '%s': assuming dirty commit", contextDir)
	}

	return s.commit, s.err
}

func (r gitRepoController) calculateLatestDirCommit(ctx context.Context, contextDir string) (string, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to calculate latestDirCommit: failed to analyze git repo: %w", err)
	}

	return repo.GetLatestCommit(ctx, r.path, contextDir, NewLocalGit("git", &LocalExec{}))
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
	return oktetoGitRepository{
		repo: repo,
		root: path,
	}, nil
}

type oktetoGitRepository struct {
	repo *git.Repository
	root string
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

func (ogr oktetoGitRepository) Log(o *git.LogOptions) (object.CommitIter, error) {
	return ogr.repo.Log(o)
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
	Log(o *git.LogOptions) (object.CommitIter, error)
	GetLatestCommit(ctx context.Context, repoPath, dirpath string, localGit LocalGitInterface) (string, error)
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

func (ogr oktetoGitRepository) GetLatestCommit(ctx context.Context, repoPath, dirpath string, localGit LocalGitInterface) (string, error) {
	// using git directly is faster, so we check if it's available
	_, err := localGit.Exists()
	if err != nil {
		// git is not available, so we fall back on git-go
		oktetoLog.Debug("Calculating git latest commit: git is not installed, for better performances consider installing it")
		cIter, err := ogr.Log(&git.LogOptions{
			PathFilter: func(s string) bool {
				return s == dirpath
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to get git log: %w", err)
		}
		commit := ""
		err = cIter.ForEach(func(c *object.Commit) error {
			commit = c.Hash.String()
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to get git log: %w", err)
		}
		return commit, nil
	}

	commit, err := localGit.GetLatestCommit(ctx, ogr.root, dirpath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}

	return commit, nil
}

func FindTopLevelGitDir(workingDir string, fs afero.Fs) (string, error) {
	dir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("%w: invalid workind dir: %w", errFindingRepo, err)
	}

	for {
		if filesystem.FileExistsWithFilesystem(filepath.Join(dir, ".git"), fs) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errFindingRepo
		}
		dir = parent
	}
}
