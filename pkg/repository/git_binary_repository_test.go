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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/stretchr/testify/require"
)

// requireGitBinary skips the test when git is not installed, since it checks
// how the repository code reads repositories with the git binary.
func requireGitBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// initTestRepoWithCommit creates a test repository with one committed file in
// api/ and returns its path and worktree.
func initTestRepoWithCommit(t *testing.T) (string, *git.Worktree) {
	t.Helper()
	dir, worktree := initTestRepo(t)
	writeRepoFile(t, dir, "api/main.go", "package main")
	commitAll(t, worktree, "initial")
	return dir, worktree
}

func addRemotes(t *testing.T, dir string, remotes ...*config.RemoteConfig) {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	for _, remote := range remotes {
		_, err := repo.CreateRemote(remote)
		require.NoError(t, err)
	}
}

func TestGitRepositoryGetter_readsWithGitBinary(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepoWithCommit(t)

	repo, err := gitRepositoryGetter{}.get(dir)
	require.NoError(t, err)
	require.IsType(t, gitBinaryRepository{}, repo)
}

func TestNewGitBinaryRepository_rootFromSubdir(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepoWithCommit(t)

	repo, err := newGitBinaryRepository(&LocalExec{}, filepath.Join(dir, "api"))
	require.NoError(t, err)
	require.Equal(t, dir, repo.root)
}

// The okteto up branch tracking and the repository URL of deploy and up look
// for the repository from the manifest path, as go-git allows.
func TestNewGitBinaryRepository_rootFromFile(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepoWithCommit(t)

	repo, err := newGitBinaryRepository(&LocalExec{}, filepath.Join(dir, "api", "main.go"))
	require.NoError(t, err)
	require.Equal(t, dir, repo.root)
}

func TestNewGitBinaryRepository_notARepository(t *testing.T) {
	requireGitBinary(t)

	_, err := newGitBinaryRepository(&LocalExec{}, t.TempDir())
	require.Error(t, err)
}

func TestNewGitBinaryRepository_missingPath(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepoWithCommit(t)

	_, err := newGitBinaryRepository(&LocalExec{}, filepath.Join(dir, "missing"))
	require.Error(t, err)
}

func TestGitBinaryRepositoryHead_branchMatchesGoGit(t *testing.T) {
	requireGitBinary(t)
	dir, worktree := initTestRepoWithCommit(t)
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("feature/x"), Create: true}))
	goGitRepo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	want, err := goGitRepo.Head()
	require.NoError(t, err)

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	head, err := repo.Head()
	require.NoError(t, err)
	require.Equal(t, want.Name(), head.Name())
	require.Equal(t, want.Hash(), head.Hash())
}

func TestGitBinaryRepositoryHead_detachedMatchesGoGit(t *testing.T) {
	requireGitBinary(t)
	dir, worktree := initTestRepoWithCommit(t)
	goGitRepo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	branchHead, err := goGitRepo.Head()
	require.NoError(t, err)
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{Hash: branchHead.Hash()}))
	want, err := goGitRepo.Head()
	require.NoError(t, err)

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	head, err := repo.Head()
	require.NoError(t, err)
	require.Equal(t, plumbing.HEAD, head.Name())
	require.Equal(t, want.Name(), head.Name())
	require.Equal(t, want.Hash(), head.Hash())
}

func TestGitBinaryRepositoryHead_noCommits(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepo(t)

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	_, err = repo.Head()
	require.Error(t, err)
}

func TestGitBinaryRepositoryRemoteURL_prefersOrigin(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepo(t)
	addRemotes(t, dir,
		&config.RemoteConfig{Name: "fork", URLs: []string{"https://github.com/fork/repo.git"}},
		&config.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:okteto/repo.git", "https://github.com/okteto/repo.git"}},
	)

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	url, err := repo.RemoteURL()
	require.NoError(t, err)
	require.Equal(t, "git@github.com:okteto/repo.git", url)
}

func TestGitBinaryRepositoryRemoteURL_firstRemoteWithoutOrigin(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepo(t)
	addRemotes(t, dir,
		&config.RemoteConfig{Name: "upstream", URLs: []string{"https://github.com/okteto/repo.git"}},
		&config.RemoteConfig{Name: "fork", URLs: []string{"https://github.com/fork/repo.git"}},
	)

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	url, err := repo.RemoteURL()
	require.NoError(t, err)
	require.Equal(t, "https://github.com/okteto/repo.git", url)
}

func TestGitBinaryRepositoryRemoteURL_noRemotes(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepo(t)

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	_, err = repo.RemoteURL()
	require.ErrorIs(t, err, errNoRemote)
}

// With the worktreeConfig extension, a remote can live in config.worktree,
// which go-git merges into the repository's config.
func TestGitBinaryRepositoryRemoteURL_remoteInWorktreeConfig(t *testing.T) {
	requireGitBinary(t)
	dir, _ := initTestRepo(t)
	goGitRepo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	cfg, err := goGitRepo.Config()
	require.NoError(t, err)
	cfg.Raw.Section("extensions").SetOption("worktreeConfig", "true")
	require.NoError(t, goGitRepo.SetConfig(cfg))
	writeRepoFile(t, dir, ".git/config.worktree", "[remote \"origin\"]\n\turl = https://github.com/okteto/repo.git\n")

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	url, err := repo.RemoteURL()
	require.NoError(t, err)
	require.Equal(t, "https://github.com/okteto/repo.git", url)
}

// git remote get-url would apply the insteadOf rules of the user's global git
// config, which go-git doesn't read, and the URL is sent to Okteto.
func TestGitBinaryRepositoryRemoteURL_ignoresGlobalInsteadOf(t *testing.T) {
	requireGitBinary(t)
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(globalConfig, []byte("[url \"git@github.com:\"]\n\tinsteadOf = https://github.com/\n"), 0o600))
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	dir, _ := initTestRepo(t)
	addRemotes(t, dir, &config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/okteto/repo.git"}})

	repo, err := newGitBinaryRepository(&LocalExec{}, dir)
	require.NoError(t, err)
	url, err := repo.RemoteURL()
	require.NoError(t, err)
	require.Equal(t, "https://github.com/okteto/repo.git", url)
}

func TestGetBranch_detachedHead(t *testing.T) {
	requireGitBinary(t)
	dir, worktree := initTestRepoWithCommit(t)
	goGitRepo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	head, err := goGitRepo.Head()
	require.NoError(t, err)
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{Hash: head.Hash()}))

	_, err = GetBranch(dir)
	require.Error(t, err)
}

func TestRepository_readsMetadataWithGitBinary(t *testing.T) {
	requireGitBinary(t)
	dir, worktree := initTestRepoWithCommit(t)
	addRemotes(t, dir, &config.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:okteto/repo.git"}})
	goGitRepo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	head, err := goGitRepo.Head()
	require.NoError(t, err)
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("main"), Create: true}))

	repo := NewRepository(dir)
	sha, err := repo.GetSHA()
	require.NoError(t, err)
	require.Equal(t, head.Hash().String(), sha)
	branch, err := repo.GetCurrentBranch()
	require.NoError(t, err)
	require.Equal(t, "main", branch)
	require.Equal(t, "https://github.com/okteto/repo", repo.GetAnonymizedRepo())
}
