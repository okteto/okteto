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

//go:build !windows

package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newSymlinkedRepo creates a git repository with a commit at <tmp>/real/repo
// and a symlink to it at <tmp>/work/repo, and returns both paths. The
// symlink's target lies outside the symlink's parent directory, as when a user
// works from a symlinked checkout.
func newSymlinkedRepo(t *testing.T) (link, target string) {
	t.Helper()
	tmp := t.TempDir()

	target = filepath.Join(tmp, "real", "repo")
	worktree := initTestRepoAt(t, target)
	writeRepoFile(t, target, "api/main.go", "package main")
	commitAll(t, worktree, "initial")

	require.NoError(t, os.Mkdir(filepath.Join(tmp, "work"), 0o700))
	link = filepath.Join(tmp, "work", "repo")
	require.NoError(t, os.Symlink(filepath.Join("..", "real", "repo"), link))

	return link, target
}

// TestRepository_symlinkedWorkingDir reproduces the failure seen when running
// okteto from a working directory that is a symlink to a repository outside
// the symlink's parent directory. go-git v6 refuses to open it with "path
// escapes from parent", so Smart Builds couldn't read the git metadata and
// rebuilt every image. With git installed, the git binary reads it instead.
func TestRepository_symlinkedWorkingDir(t *testing.T) {
	requireGitBinary(t)
	link, target := newSymlinkedRepo(t)
	fromLink := NewRepository(link)
	fromTarget := NewRepository(target)

	sha, err := fromLink.GetSHA()
	require.NoError(t, err)
	wantSHA, err := fromTarget.GetSHA()
	require.NoError(t, err)
	require.Equal(t, wantSHA, sha)

	dirSHA, err := fromLink.GetLatestDirSHA(filepath.Join(link, "api"))
	require.NoError(t, err)
	wantDirSHA, err := fromTarget.GetLatestDirSHA(filepath.Join(target, "api"))
	require.NoError(t, err)
	require.Equal(t, wantDirSHA, dirSHA)
}

// Smart Builds also hashes the uncommitted changes of the build context, and
// it must see the same changes from the symlink as from the real path.
func TestRepository_symlinkedWorkingDirSeesUncommittedChanges(t *testing.T) {
	requireGitBinary(t)
	link, target := newSymlinkedRepo(t)
	fromLink := NewRepository(link)
	fromTarget := NewRepository(target)
	cleanHash, err := fromTarget.GetDiffHash(filepath.Join(target, "api"))
	require.NoError(t, err)

	writeRepoFile(t, target, "api/main.go", "package main\n\nfunc main() {}\n")
	writeRepoFile(t, target, "api/new.go", "package main\n")

	diffHash, err := fromLink.GetDiffHash(filepath.Join(link, "api"))
	require.NoError(t, err)
	wantDiffHash, err := fromTarget.GetDiffHash(filepath.Join(target, "api"))
	require.NoError(t, err)
	require.Equal(t, wantDiffHash, diffHash)
	require.NotEqual(t, cleanHash, diffHash)
}

// TestFindTopLevelGitDir_symlinkedWorkingDir covers the same scenario for the
// repository detection of okteto deploy, which otherwise warns "Repository not
// detected" and leaves OKTETO_GIT_COMMIT and OKTETO_GIT_BRANCH unset.
func TestFindTopLevelGitDir_symlinkedWorkingDir(t *testing.T) {
	link, target := newSymlinkedRepo(t)

	dir, err := FindTopLevelGitDir(link)
	require.NoError(t, err)

	got, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// The root keeps the form of the path the caller passed: deploy -f and the
// Smart Builds ignore filters compare it textually with paths built from the
// logical working dir.
func TestFindTopLevelGitDir_keepsSymlinkedWorkingDirForm(t *testing.T) {
	requireGitBinary(t)
	link, _ := newSymlinkedRepo(t)

	dir, err := FindTopLevelGitDir(filepath.Join(link, "api"))
	require.NoError(t, err)
	require.Equal(t, link, dir)
}

// newRepoUnderSymlinkedParent creates a git repository at
// <tmp>/real/parent/repo and returns its path through the symlink
// <tmp>/alias, which points to <tmp>/real/parent. go-git opens these, and
// returns the root in the form of the path it got.
func newRepoUnderSymlinkedParent(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	target := filepath.Join(tmp, "real", "parent", "repo")
	worktree := initTestRepoAt(t, target)
	writeRepoFile(t, target, "api/main.go", "package main")
	commitAll(t, worktree, "initial")

	alias := filepath.Join(tmp, "alias")
	require.NoError(t, os.Symlink(filepath.Join(tmp, "real", "parent"), alias))
	return filepath.Join(alias, "repo")
}

func TestFindTopLevelGitDir_keepsSymlinkedParentForm(t *testing.T) {
	requireGitBinary(t)
	repo := newRepoUnderSymlinkedParent(t)

	dir, err := FindTopLevelGitDir(filepath.Join(repo, "api"))
	require.NoError(t, err)
	require.Equal(t, repo, dir)
}

func TestFindTopLevelGitDir_goGitFallbackKeepsSymlinkedParentForm(t *testing.T) {
	withoutGitBinary(t)
	repo := newRepoUnderSymlinkedParent(t)

	dir, err := FindTopLevelGitDir(filepath.Join(repo, "api"))
	require.NoError(t, err)
	require.Equal(t, repo, dir)
}

// A symlink inside the repository that points to a deeper dir: git counts the
// levels up to the top level from the resolved dir, so joining
// git rev-parse --show-cdup to the caller's path would go above the
// repository.
func TestFindTopLevelGitDir_symlinkInsideRepository(t *testing.T) {
	requireGitBinary(t)
	dir, worktree := initTestRepo(t)
	writeRepoFile(t, dir, "services/api/main.go", "package main")
	commitAll(t, worktree, "initial")
	require.NoError(t, os.Symlink(filepath.Join("services", "api"), filepath.Join(dir, "api")))

	root, err := FindTopLevelGitDir(filepath.Join(dir, "api"))
	require.NoError(t, err)
	require.Equal(t, dir, root)
}

// A symlink from outside the repository to one of its dirs: no parent of the
// caller's path is the repository, so the root is git's resolved top level.
func TestFindTopLevelGitDir_symlinkIntoRepositoryFromOutside(t *testing.T) {
	requireGitBinary(t)
	dir, worktree := initTestRepo(t)
	writeRepoFile(t, dir, "services/api/main.go", "package main")
	commitAll(t, worktree, "initial")
	shortcut := filepath.Join(t.TempDir(), "api")
	require.NoError(t, os.Symlink(filepath.Join(dir, "services", "api"), shortcut))
	want, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	root, err := FindTopLevelGitDir(shortcut)
	require.NoError(t, err)
	require.Equal(t, want, root)
}
