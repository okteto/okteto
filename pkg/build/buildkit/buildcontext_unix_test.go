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

package buildkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSmartBuildContextOfASymlinkToTheFolder(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(c.contextPath, link))
	c.contextPath = link
	second := c.configure(t)

	require.NotEmpty(t, first.SharedKey)
	require.Equal(t, first.SharedKey, second.SharedKey)
	require.Equal(t, first.FrontendAttrs[testSourceOpt], second.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextOfARepositoryReachedThroughASymlink(t *testing.T) {
	c := newTestBuildContext(t)
	first := c.configure(t)

	// the root of the repository always comes without symbolic links
	link := filepath.Join(t.TempDir(), "repository")
	require.NoError(t, os.Symlink(c.root, link))
	c.contextPath = filepath.Join(link, "services", "api")
	c.dockerfile = filepath.Join(c.contextPath, "Dockerfile")
	second := c.configure(t)

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, ""), second.SharedKey)
	require.Equal(t, first.SharedKey, second.SharedKey)
	require.Equal(t, first.FrontendAttrs[testSourceOpt], second.FrontendAttrs[testSourceOpt])
}

func TestSmartBuildContextKeyOfADockerfileThatIsASymlink(t *testing.T) {
	c := newTestBuildContext(t)
	writeTestFile(t, c.dockerfile, "FROM scratch\n")
	first := c.configure(t)

	link := filepath.Join(c.root, "Dockerfile.link")
	require.NoError(t, os.Symlink(c.dockerfile, link))
	c.dockerfile = link
	second := c.configure(t)

	require.Equal(t, first.SharedKey, second.SharedKey)
}

func TestResolveSymlinksOfAPathThatDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(dir, link))

	resolved := resolveSymlinks(filepath.Join(link, "missing", "Dockerfile"))

	require.Equal(t, filepath.Join(resolvedDir, "missing", "Dockerfile"), resolved)
}

func TestSmartBuildContextOfASymlinkOutsideTheRepository(t *testing.T) {
	root := newTestGitRepository(t, "git@github.com:okteto/movies.git")
	contextPath := filepath.Join(root, "services", "api")
	link := filepath.Join(t.TempDir(), "api")
	require.NoError(t, os.Symlink(contextPath, link))

	direct := configureRealBuildContext(t, contextPath)
	linked := configureRealBuildContext(t, link)

	require.Equal(t, testKey(testContextFolder, "", testDockerfile, ""), direct.SharedKey)
	require.Equal(t, direct.SharedKey, linked.SharedKey)
}

func TestUploadedFolderResolvesSymlinksBeforeParentFolders(t *testing.T) {
	// "link/.." is the parent of the folder that the link points to, not the folder that has the link
	target := filepath.Join(t.TempDir(), "repository", "services")
	require.NoError(t, os.MkdirAll(target, testDirPermission))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))
	expected, err := filepath.EvalSymlinks(filepath.Dir(target))
	require.NoError(t, err)

	folder, err := uploadedFolder(link + string(filepath.Separator) + "..")

	require.NoError(t, err)
	require.Equal(t, expected, folder)
}
