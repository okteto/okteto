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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	giturls "github.com/chainguard-dev/git-urls"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
)

const (
	gitOperationTimeout = 5 * time.Second
)

var (
	errNotCleanRepo    = errors.New("repository is not clean")
	errTimeoutExceeded = errors.New("timeout exceeded")
	errFindingRepo     = errors.New("top level git repo directory cannot be found")
)

type gitRepoController struct {
	repoGetter repositoryGetterInterface
	fs         afero.Fs
	path       string
}

func newGitRepoController(path string) gitRepoController {
	return gitRepoController{
		repoGetter: gitRepositoryGetter{},
		path:       path,
		fs:         afero.NewOsFs(),
	}
}

type cleanStatus struct {
	err     error
	isClean bool
}

func (r gitRepoController) getCurrentBranch() (string, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}
	return head.Name().Short(), nil
}

func (r gitRepoController) calculateIsClean(ctx context.Context) (bool, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return false, fmt.Errorf("failed to analyze git repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's current worktree: %w", err)
	}

	status, err := worktree.Status(ctx, "", NewLocalGit("git", &LocalExec{}))
	if err != nil {
		return false, fmt.Errorf("failed to infer the git repo's status: %w", err)
	}

	return status.IsClean(), nil
}

func (r gitRepoController) getRepoURL() (string, error) {
	url, err := giturls.Parse(r.path)
	if err != nil {
		oktetoLog.Infof("could not parse url: %s", err)
	}
	if url.Scheme != "file" {
		return r.sanitiseURL(url.String()), nil
	}

	repo, err := FindTopLevelGitRepoFromPath(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}
	origin, err := repo.Remote("origin")
	if err != nil {
		if err != git.ErrRemoteNotFound {
			return "", fmt.Errorf("failed to get the git repo's remote configuration: %w", err)
		}
	}

	if origin != nil {
		return r.sanitiseURL(origin.Config().URLs[0]), nil
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("failed to get git repo's remote information: %w", err)
	}

	if len(remotes) == 0 {
		return "", fmt.Errorf("git repo doesn't have any remote")
	}

	return r.sanitiseURL(remotes[0].Config().URLs[0]), nil
}

func (r gitRepoController) sanitiseURL(u string) string {
	url, err := giturls.Parse(u)
	if err != nil {
		oktetoLog.Infof("could not parse url: %s", err)
		return u
	}
	url.Path = strings.Replace(url.Path, "//", "/", -1)
	return url.String()
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

// GetLatestDirSHA It calculates a sha of the contextDir specified as parameter based on the content
func (r gitRepoController) GetLatestDirSHA(contextDir string) (string, error) {
	// We use context.TODO() in a few places to call isClean, so let's make sure
	// we set proper internal timeouts to not leak goroutines
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	timeoutCh := make(chan struct{})
	ch := make(chan commitResponse)

	go func() {
		time.Sleep(gitOperationTimeout)
		close(timeoutCh)
		cancel()
		ch <- commitResponse{
			commit: "",
			err:    errTimeoutExceeded,
		}
	}()

	go func() {
		commit, err := r.calculateLatestDirSHA(ctx, contextDir)
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

type diffResponse struct {
	err  error
	diff string
}

type untrackedFilesResponse struct {
	err                error
	untrackedFilesDiff string
}

func (r gitRepoController) GetDiffHash(contextDir string) (string, error) {
	// We use context.TODO() in a few places to call GetDiffHash, so let's make sure
	// we set proper internal timeouts to not leak goroutines
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	timeoutCh := make(chan struct{})
	diffCh := make(chan diffResponse)
	untrackedFilesCh := make(chan untrackedFilesResponse)

	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}

	// go func that cancels the context after the timeout
	go func() {
		time.Sleep(gitOperationTimeout)
		close(timeoutCh)
		cancel()
		diffCh <- diffResponse{
			diff: "",
			err:  errTimeoutExceeded,
		}
		untrackedFilesCh <- untrackedFilesResponse{
			untrackedFilesDiff: "",
			err:                errTimeoutExceeded,
		}
	}()

	// go func that calculates the diff using git diff
	go func() {
		diff, err := repo.GetDiff(ctx, contextDir, NewLocalGit("git", &LocalExec{}))
		select {
		case <-timeoutCh:
			oktetoLog.Debug("Timeout exceeded calculating git diff: assuming dirty commit")
		case diffCh <- diffResponse{
			diff: diff,
			err:  err,
		}:
		}
	}()

	// go func that calculates the diff for the untracked files
	go func() {
		untrackedFiles, err := repo.calculateUntrackedFiles(ctx, contextDir)
		if err != nil {
			select {
			case <-timeoutCh:
				oktetoLog.Debug("Timeout exceeded calculating git untracked files: assuming dirty commit")
			case untrackedFilesCh <- untrackedFilesResponse{
				untrackedFilesDiff: "",
				err:                err,
			}:
			}
			return
		}

		untrackedFilesContent, err := r.getUntrackedContent(ctx, untrackedFiles)
		select {
		case <-timeoutCh:
			oktetoLog.Debug("Timeout exceeded calculating git untracked files: assuming dirty commit")
		case untrackedFilesCh <- untrackedFilesResponse{
			untrackedFilesDiff: untrackedFilesContent,
			err:                err,
		}:
		}
	}()

	diffResponse := <-diffCh
	if diffResponse.err != nil {
		return "", diffResponse.err
	}

	untrackedFilesResponse := <-untrackedFilesCh
	if untrackedFilesResponse.err != nil {
		return "", untrackedFilesResponse.err
	}

	hashFrom := fmt.Sprintf("%s-%s", diffResponse.diff, untrackedFilesResponse.untrackedFilesDiff)
	oktetoLog.Infof("hashing diff: %s", hashFrom)
	diffHash := sha256.Sum256([]byte(hashFrom))
	return hex.EncodeToString(diffHash[:]), nil
}

func (r gitRepoController) getUntrackedContent(ctx context.Context, files []string) (string, error) {
	totalContent := ""
	for _, file := range files {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			absPath := filepath.Join(r.path, file)
			content, err := afero.ReadFile(r.fs, absPath)
			if err != nil {
				return "", fmt.Errorf("failed to read file '%s': %w", absPath, err)
			}
			totalContent += fmt.Sprintf("%s:%s\n", file, content)
		}
	}
	return totalContent, nil
}

func (r gitRepoController) calculateLatestDirSHA(ctx context.Context, contextDir string) (string, error) {
	repo, err := r.repoGetter.get(r.path)
	if err != nil {
		return "", fmt.Errorf("failed to calculate latestDirCommit: failed to analyze git repo: %w", err)
	}

	return repo.GetLatestSHA(ctx, contextDir, NewLocalGit("git", &LocalExec{}))
}

type repositoryGetterInterface interface {
	get(path string) (gitRepositoryInterface, error)
}

type gitRepositoryGetter struct{}

func (gitRepositoryGetter) get(path string) (gitRepositoryInterface, error) {
	repo, err := FindTopLevelGitRepoFromPath(path)
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

func (ogr oktetoGitRepository) calculateUntrackedFiles(ctx context.Context, contextDir string) ([]string, error) {
	worktree, err := ogr.Worktree()
	if err != nil {
		return []string{}, fmt.Errorf("failed to infer the git repo's current worktree: %w", err)
	}

	return worktree.ListUntrackedFiles(ctx, contextDir, NewLocalGit("git", &LocalExec{}))
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
	GetLatestSHA(ctx context.Context, dirpath string, localGit LocalGitInterface) (string, error)
	GetDiff(ctx context.Context, dirpath string, localGit LocalGitInterface) (string, error)
	calculateUntrackedFiles(ctx context.Context, contextDir string) ([]string, error)
}

type gitWorktreeInterface interface {
	Status(context.Context, string, LocalGitInterface) (oktetoGitStatus, error)
	GetRoot() string
	ListUntrackedFiles(context.Context, string, LocalGitInterface) ([]string, error)
}

func (ogr oktetoGitWorktree) Status(ctx context.Context, repoRoot string, localGit LocalGitInterface) (oktetoGitStatus, error) {
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

	status, err := localGit.Status(ctx, ogr.GetRoot(), repoRoot, 0)
	if err != nil {
		return oktetoGitStatus{status: git.Status{}}, fmt.Errorf("failed to get git status: %w", err)
	}

	return oktetoGitStatus{status: status}, nil
}

func (ogr oktetoGitWorktree) ListUntrackedFiles(ctx context.Context, workdir string, localGit LocalGitInterface) ([]string, error) {
	// using git directly is faster, so we check if it's available
	_, err := localGit.Exists()
	if err != nil {
		// git is not available, so we fall back on git-go
		oktetoLog.Debug("Calculating git untrackedFiles: git is not installed, for better performances consider installing it")

		status, err := ogr.Status(ctx, ogr.GetRoot(), NewLocalGit("git", &LocalExec{}))
		if err != nil {
			return []string{}, fmt.Errorf("failed to infer the git repo's status: %w", err)
		}

		files := []string{}
		for k, v := range status.status {
			if v.Staging == git.Untracked {
				files = append(files, k)
			}
		}

		sort.Strings(files)
		return files, nil
	}

	untrackedFiles, err := localGit.ListUntrackedFiles(ctx, ogr.GetRoot(), workdir, 0)
	if err != nil {
		return []string{}, fmt.Errorf("failed to get untrackedFiles: %w", err)
	}

	return untrackedFiles, nil
}

// GetLatestSHA calculates a SHA of a repo for the specified directory (dirpath). The result depends on having
// git installed locally or not.
// - If git is not installed locally, it will use the latest commit of that directory.
// - If git is installed it will get a SHA from the files tracked within that directory.
func (ogr oktetoGitRepository) GetLatestSHA(ctx context.Context, dirpath string, localGit LocalGitInterface) (string, error) {
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

	commit, err := localGit.GetDirContentSHA(ctx, ogr.root, dirpath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}

	return commit, nil
}

// GetDiff returns the diff between the current state of the repo and the last commit
func (ogr oktetoGitRepository) GetDiff(ctx context.Context, dirpath string, localGit LocalGitInterface) (string, error) {
	// using git directly is faster, so we check if it's available
	_, err := localGit.Exists()
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}

	diff, err := localGit.Diff(ctx, ogr.root, dirpath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}

	return diff, nil
}

// FindTopLevelGitDir returns the top level git directory for the given working directory
func FindTopLevelGitDir(workingDir string) (string, error) {
	dir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("%w: invalid workind dir: %w", errFindingRepo, err)
	}

	repo, err := FindTopLevelGitRepoFromPath(dir)
	if err != nil {
		return "", fmt.Errorf("%w: failed to find top level git repo: %w", errFindingRepo, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("%w: failed to find top level git repo: %w", errFindingRepo, err)
	}

	return worktree.Filesystem.Root(), nil
}

// FindTopLevelGitRepoFromPath returns the git repository for the given absolute path taking into parent folders
func FindTopLevelGitRepoFromPath(path string) (*git.Repository, error) {
	opts := &git.PlainOpenOptions{
		DetectDotGit: true,
	}
	return git.PlainOpenWithOptions(path, opts)
}
