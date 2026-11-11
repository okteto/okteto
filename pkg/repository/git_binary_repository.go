// Copyright 2026 The Okteto Authors
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6/plumbing"
)

const originRemoteURLKey = "remote.origin.url"

var errNoRemote = errors.New("git repo doesn't have any remote")

// gitBinaryRepository reads a repository with the git binary, so go-git never opens it
type gitBinaryRepository struct {
	executor CommandExecutor
	gitPath  string

	// path is where the caller looked for the repository. Dir hashes and diffs run from it, as in oktetoGitRepository
	path string

	// root is the repository's top level dir, in the form of path
	root string
}

// newGitBinaryRepository finds the repository that contains path with the git binary. It fails when git is not
// installed or doesn't recognize the repository
func newGitBinaryRepository(executor CommandExecutor, path string) (gitBinaryRepository, error) {
	gitPath, err := executor.LookPath("git")
	if err != nil {
		return gitBinaryRepository{}, err
	}

	dir, err := filepath.Abs(path)
	if err != nil {
		return gitBinaryRepository{}, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return gitBinaryRepository{}, err
	}
	// Callers also pass manifest paths, and go-git looks for the repository from the file's dir
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	toplevel, err := runGit(context.Background(), executor, dir, gitPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return gitBinaryRepository{}, err
	}
	root, err := rootInPathForm(dir, filepath.FromSlash(toplevel))
	if err != nil {
		return gitBinaryRepository{}, err
	}

	return gitBinaryRepository{
		executor: executor,
		gitPath:  gitPath,
		path:     path,
		root:     root,
	}, nil
}

// rootInPathForm returns the repository's top level dir in the form of dir. git prints it with symlinks
// resolved, but callers compare it textually with paths built from the working dir, like the manifest path that
// deploy stores or the Smart Builds ignore filters. So the top level is looked up among dir and its parents, and the
// resolved path is returned only when none of them is the top level, as when dir is a symlink from outside the
// repository
func rootInPathForm(dir, toplevel string) (string, error) {
	toplevelInfo, err := os.Stat(toplevel)
	if err != nil {
		return "", err
	}

	for candidate := dir; ; candidate = filepath.Dir(candidate) {
		if info, err := os.Stat(candidate); err == nil && os.SameFile(info, toplevelInfo) {
			return candidate, nil
		}
		if filepath.Dir(candidate) == candidate {
			return toplevel, nil
		}
	}
}

// runGit runs a git command in dir and returns its output without the trailing newline. Errors carry git's
// stderr, so the logs say why git failed
func runGit(ctx context.Context, executor CommandExecutor, dir, gitPath string, args ...string) (string, error) {
	output, err := executor.RunCommand(ctx, dir, gitPath, append([]string{"--no-optional-locks"}, args...)...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}

func (r gitBinaryRepository) Worktree() (gitWorktreeInterface, error) {
	return gitBinaryWorktree{root: r.root}, nil
}

// Head returns the commit HEAD points to, named after its branch, or HEAD when it is detached, as go-git does
func (r gitBinaryRepository) Head() (*plumbing.Reference, error) {
	// --symbolic-full-name applies only to the second HEAD, so git prints the commit first and then the ref name
	output, err := runGit(context.Background(), r.executor, r.root, r.gitPath, "rev-parse", "HEAD", "--symbolic-full-name", "HEAD")
	if err != nil {
		return nil, err
	}
	hash, name, found := strings.Cut(output, "\n")
	if !found {
		return nil, fmt.Errorf("unexpected git rev-parse output: %q", output)
	}
	return plumbing.NewHashReference(plumbing.ReferenceName(strings.TrimSpace(name)), plumbing.NewHash(strings.TrimSpace(hash))), nil
}

// RemoteURL returns the first URL of the origin remote, or the first remote URL in git's config when there is no
// origin. It returns the URLs as written in the config, because the URL is sent to Okteto and git remote get-url
// would rewrite it with the user's insteadOf rules. It reads every config file, not only the repository's
// .git/config, so that remotes defined in config.worktree are found too
func (r gitBinaryRepository) RemoteURL() (string, error) {
	output, err := runGit(context.Background(), r.executor, r.root, r.gitPath, "config", "--null", "--get-regexp", `^remote\..*\.url$`)
	if err != nil {
		var exitErr *exec.ExitError
		// git config exits with 1 when no key matches
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", errNoRemote
		}
		return "", fmt.Errorf("failed to get git repo's remote information: %w", err)
	}

	firstURL := ""
	// --null ends each entry with a NUL and separates the key from the value with a newline
	for _, entry := range strings.Split(output, "\x00") {
		key, url, found := strings.Cut(entry, "\n")
		if !found {
			continue
		}
		if key == originRemoteURLKey {
			return url, nil
		}
		if firstURL == "" {
			firstURL = url
		}
	}
	if firstURL == "" {
		return "", errNoRemote
	}
	return firstURL, nil
}

func (r gitBinaryRepository) GetLatestSHA(ctx context.Context, dirpath string, localGit LocalGitInterface) (string, error) {
	commit, err := localGit.GetDirContentSHA(ctx, r.path, dirpath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get git dir content SHA: %w", err)
	}
	return commit, nil
}

func (r gitBinaryRepository) GetDiff(ctx context.Context, dirpath string, localGit LocalGitInterface) (string, error) {
	diff, err := localGit.Diff(ctx, r.path, dirpath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get git diff: %w", err)
	}
	return diff, nil
}

func (r gitBinaryRepository) calculateUntrackedFiles(ctx context.Context, contextDir string) ([]string, error) {
	return gitBinaryWorktree{root: r.root}.ListUntrackedFiles(ctx, contextDir, NewLocalGit(r.gitPath, r.executor, nil, false))
}

// gitBinaryWorktree is the worktree of a gitBinaryRepository
type gitBinaryWorktree struct {
	root string
}

func (w gitBinaryWorktree) GetRoot() string {
	return w.root
}

func (w gitBinaryWorktree) Status(ctx context.Context, dirPath string, localGit LocalGitInterface) (oktetoGitStatus, error) {
	status, err := localGit.Status(ctx, w.root, dirPath, 0)
	if err != nil {
		return oktetoGitStatus{}, fmt.Errorf("failed to get git status: %w", err)
	}
	return oktetoGitStatus{status: status}, nil
}

func (w gitBinaryWorktree) ListUntrackedFiles(ctx context.Context, workdir string, localGit LocalGitInterface) ([]string, error) {
	untrackedFiles, err := localGit.ListUntrackedFiles(ctx, w.root, workdir, 0)
	if err != nil {
		return []string{}, fmt.Errorf("failed to get untrackedFiles: %w", err)
	}
	return untrackedFiles, nil
}
