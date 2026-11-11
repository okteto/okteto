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

	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/require"
)

// newSymlinkedRepo creates a git repository at <tmp>/real/repo and a symlink
// to it at <tmp>/work/repo, and returns both paths. The symlink's target lies
// outside the symlink's parent directory, as when a user works from a
// symlinked checkout.
func newSymlinkedRepo(t *testing.T) (link, target string) {
	t.Helper()
	tmp := t.TempDir()

	target = filepath.Join(tmp, "real", "repo")
	_, err := git.PlainInit(target, false)
	require.NoError(t, err)

	require.NoError(t, os.Mkdir(filepath.Join(tmp, "work"), 0o700))
	link = filepath.Join(tmp, "work", "repo")
	require.NoError(t, os.Symlink(filepath.Join("..", "real", "repo"), link))

	return link, target
}

// TestFindTopLevelGitRepoFromPath_symlinkedWorkingDir reproduces the failure
// seen when running okteto from a working directory that is a symlink to a
// repository outside the symlink's parent directory. go-git v6 refuses to open
// it with "path escapes from parent", so Smart Builds can't read the git
// metadata and rebuilds every image.
func TestFindTopLevelGitRepoFromPath_symlinkedWorkingDir(t *testing.T) {
	link, _ := newSymlinkedRepo(t)

	repo, err := FindTopLevelGitRepoFromPath(link)
	require.NoError(t, err)
	require.NotNil(t, repo)
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
