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

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

// TestFindTopLevelGitRepoFromPath_linkedWorktree covers running okteto from a
// worktree created with "git worktree add". Its .git is a file pointing to
// .git/worktrees/<name> in the main repository, and the remotes and branches
// live in the main repository's .git, which the "commondir" file points to.
//
// It doesn't run on Windows: go-git v5.16.5 opens "commondir" and never closes
// it (fixed in v5.19.3, see go.mod for why the CLI stays on 5.16.5), and
// Windows can't delete an open file, so removing the test's temp dir fails.
func TestFindTopLevelGitRepoFromPath_linkedWorktree(t *testing.T) {
	tmp := t.TempDir()
	mainDir := filepath.Join(tmp, "main")
	mainRepo, err := git.PlainInit(mainDir, false)
	require.NoError(t, err)
	_, err = mainRepo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/okteto/movies.git"}})
	require.NoError(t, err)
	mainWorktree, err := mainRepo.Worktree()
	require.NoError(t, err)
	writeRepoFile(t, mainDir, "README.md", "movies")
	head := commitAll(t, mainWorktree, "initial")
	feature := plumbing.NewBranchReferenceName("feature")
	require.NoError(t, mainRepo.Storer.SetReference(plumbing.NewHashReference(feature, plumbing.NewHash(head))))

	// Lay out the linked worktree the way "git worktree add -b feature <tmp>/wt"
	// does. Its HEAD is its own, so it is on "feature" while the main
	// repository stays on "master".
	worktreeDir := filepath.Join(tmp, "wt")
	worktreeGitDir := filepath.Join(mainDir, ".git", "worktrees", "wt")
	require.NoError(t, os.MkdirAll(worktreeGitDir, 0o755))
	require.NoError(t, os.MkdirAll(worktreeDir, 0o755))
	writeRepoFile(t, worktreeGitDir, "HEAD", "ref: "+feature.String()+"\n")
	writeRepoFile(t, worktreeGitDir, "commondir", "../..\n")
	writeRepoFile(t, worktreeGitDir, "gitdir", filepath.Join(worktreeDir, ".git")+"\n")
	writeRepoFile(t, worktreeDir, ".git", "gitdir: "+worktreeGitDir+"\n")

	repo, err := FindTopLevelGitRepoFromPath(worktreeDir)
	require.NoError(t, err)

	origin, err := repo.Remote("origin")
	require.NoError(t, err)
	require.Equal(t, []string{"https://github.com/okteto/movies.git"}, origin.Config().URLs)

	ref, err := repo.Head()
	require.NoError(t, err)
	require.Equal(t, feature, ref.Name())
	require.Equal(t, head, ref.Hash().String())
}
