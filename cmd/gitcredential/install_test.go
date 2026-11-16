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

package gitcredential

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testHelper = "!/okteto/bin/okteto git-credential"

// fakeGit records the git commands it runs. It answers "git var" with varOutput
// and fails the commands whose args start with failOn.
type fakeGit struct {
	err       error
	varOutput string
	failOn    string
	calls     []string
}

func (f *fakeGit) run(_ context.Context, args ...string) (string, error) {
	call := strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.failOn != "" && strings.HasPrefix(call, f.failOn) {
		return "", f.err
	}
	if args[0] == "var" {
		return f.varOutput, nil
	}
	return "", nil
}

func TestInstall_Global(t *testing.T) {
	git := &fakeGit{}

	err := install(context.Background(), git.run, globalFlag, testHelper)

	require.NoError(t, err)
	require.Equal(t, []string{
		"config --global --replace-all credential.https://github.com.helper ",
		"config --global --add credential.https://github.com.helper " + testHelper,
		"config --global credential.https://github.com.useHttpPath true",
	}, git.calls)
}

func TestInstall_SystemCreatesTheConfigDirectory(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "usr", "etc", "gitconfig")
	git := &fakeGit{varOutput: configPath + "\n"}

	err := install(context.Background(), git.run, systemFlag, testHelper)

	require.NoError(t, err)
	require.DirExists(t, filepath.Dir(configPath))
	require.Equal(t, []string{
		"var GIT_CONFIG_SYSTEM",
		"config --system --replace-all credential.https://github.com.helper ",
		"config --system --add credential.https://github.com.helper " + testHelper,
		"config --system credential.https://github.com.useHttpPath true",
	}, git.calls)
}

func TestInstall_SystemWithAGitWithoutGitVar(t *testing.T) {
	git := &fakeGit{failOn: "var", err: errors.New("unknown variable GIT_CONFIG_SYSTEM")}

	err := install(context.Background(), git.run, systemFlag, testHelper)

	require.NoError(t, err)
	require.Len(t, git.calls, 4)
}

func TestInstall_StopsWhenGitConfigFails(t *testing.T) {
	errLock := errors.New("could not lock config file")
	git := &fakeGit{failOn: "config --global --add", err: errLock}

	err := install(context.Background(), git.run, globalFlag, testHelper)

	require.ErrorIs(t, err, errLock)
	require.Len(t, git.calls, 2)
}

func TestHelperCommand(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "path without special characters",
			path:     "/okteto/bin/okteto",
			expected: "!/okteto/bin/okteto git-credential",
		},
		{
			name:     "path with spaces",
			path:     "/Applications/My Tools/okteto",
			expected: "!'/Applications/My Tools/okteto' git-credential",
		},
		{
			name:     "path with a single quote",
			path:     "/home/o'brien/okteto",
			expected: `!'/home/o'\''brien/okteto' git-credential`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, helperCommand(tt.path))
		})
	}
}

// requireGit returns the path of git, and skips the test when git isn't installed
func requireGit(t *testing.T) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	return gitPath
}

// useGlobalGitConfig points git's global config to a new file with content, and
// keeps the system config out
func useGlobalGitConfig(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gitconfig")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	t.Setenv("HOME", dir)
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestInstall_WithGitReplacesTheOtherGitHubHelpers(t *testing.T) {
	git := execGit(requireGit(t))
	useGlobalGitConfig(t, "[credential \"https://github.com\"]\n\thelper = osxkeychain\n\thelper = !gh auth git-credential\n")
	ctx := context.Background()

	require.NoError(t, install(ctx, git, globalFlag, testHelper))
	require.NoError(t, install(ctx, git, globalFlag, testHelper))

	helpers, err := git(ctx, "config", "--global", "--get-all", helperKey)
	require.NoError(t, err)
	require.Equal(t, "\n"+testHelper+"\n", helpers)
	useHTTPPath, err := git(ctx, "config", "--global", "--get", useHTTPPathKey)
	require.NoError(t, err)
	require.Equal(t, "true\n", useHTTPPath)
}

func TestInstall_WithGitKeepsTheHelpersOfOtherHosts(t *testing.T) {
	git := execGit(requireGit(t))
	useGlobalGitConfig(t, "[credential \"https://gitlab.com\"]\n\thelper = store\n")
	ctx := context.Background()

	require.NoError(t, install(ctx, git, globalFlag, testHelper))

	helpers, err := git(ctx, "config", "--global", "--get-all", "credential.https://gitlab.com.helper")
	require.NoError(t, err)
	require.Equal(t, "store\n", helpers)
}
