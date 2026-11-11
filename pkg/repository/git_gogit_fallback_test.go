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
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

var errGitBinaryMissing = errors.New("git binary not found")

// gitBinaryMissing makes the repository code fall back on go-git.
type gitBinaryMissing struct{}

func (gitBinaryMissing) Status(context.Context, string, string, int) (git.Status, error) {
	return nil, errGitBinaryMissing
}

func (gitBinaryMissing) Exists() (string, error) {
	return "", errGitBinaryMissing
}

func (gitBinaryMissing) FixDubiousOwnershipConfig(string) error {
	return errGitBinaryMissing
}

func (gitBinaryMissing) parseGitStatus(string) (git.Status, error) {
	return nil, errGitBinaryMissing
}

func (gitBinaryMissing) GetDirContentSHA(context.Context, string, string, int) (string, error) {
	return "", errGitBinaryMissing
}

func (gitBinaryMissing) Diff(context.Context, string, string, int) (string, error) {
	return "", errGitBinaryMissing
}

func (gitBinaryMissing) ListUntrackedFiles(context.Context, string, string, int) ([]string, error) {
	return nil, errGitBinaryMissing
}

func writeRepoFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func commitAll(t *testing.T, worktree *git.Worktree, message string) string {
	t.Helper()
	require.NoError(t, worktree.AddWithOptions(&git.AddOptions{All: true}))
	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: "okteto", Email: "test@okteto.com", When: time.Unix(1700000000, 0)},
	})
	require.NoError(t, err)
	return hash.String()
}

func TestOktetoGitWorktreeStatus_goGitFallbackHonoursNestedGitignore(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	writeRepoFile(t, dir, ".gitignore", "*.log\n")
	writeRepoFile(t, dir, "sub/.gitignore", "build/\n")
	writeRepoFile(t, dir, "tracked.txt", "v1")
	commitAll(t, worktree, "initial")

	writeRepoFile(t, dir, "tracked.txt", "v2")
	writeRepoFile(t, dir, "new.txt", "new")
	writeRepoFile(t, dir, "app.log", "ignored by root .gitignore")
	writeRepoFile(t, dir, "sub/keep.txt", "keep")
	writeRepoFile(t, dir, "sub/build/out.bin", "ignored by sub/.gitignore")
	writeRepoFile(t, dir, "sub/deep/debug.log", "ignored by root .gitignore")

	status, err := oktetoGitWorktree{worktree: worktree}.Status(context.Background(), dir, gitBinaryMissing{})
	require.NoError(t, err)
	require.False(t, status.IsClean())
	require.ElementsMatch(t, []string{"tracked.txt", "new.txt", "sub/keep.txt"}, slices.Collect(maps.Keys(status.status)))
	require.Equal(t, git.Modified, status.status["tracked.txt"].Worktree)
	require.Equal(t, git.Untracked, status.status["new.txt"].Worktree)
}

func TestOktetoGitWorktreeStatus_goGitFallbackCleanWhenOnlyIgnoredFiles(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	writeRepoFile(t, dir, ".gitignore", "node_modules/\n")
	writeRepoFile(t, dir, "main.go", "package main")
	commitAll(t, worktree, "initial")

	writeRepoFile(t, dir, "node_modules/pkg/index.js", "ignored")

	status, err := oktetoGitWorktree{worktree: worktree}.Status(context.Background(), dir, gitBinaryMissing{})
	require.NoError(t, err)
	require.True(t, status.IsClean())
}

func TestOktetoGitRepositoryGetLatestSHA_goGitFallbackFiltersByPath(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	writeRepoFile(t, dir, "api/main.go", "package main")
	first := commitAll(t, worktree, "add api")
	writeRepoFile(t, dir, "web/index.html", "<html></html>")
	commitAll(t, worktree, "add web")

	opened, err := FindTopLevelGitRepoFromPath(filepath.Join(dir, "web"))
	require.NoError(t, err)

	sha, err := oktetoGitRepository{repo: opened, root: dir}.GetLatestSHA(context.Background(), "api/main.go", gitBinaryMissing{})
	require.NoError(t, err)
	require.Equal(t, first, sha)
}
