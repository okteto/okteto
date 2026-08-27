// Copyright 2023-2025 The Okteto Authors
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

package plugin

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// newTestRoot mirrors the registration shapes of the real root command:
// visible commands, aliased commands, hidden commands, and a hidden command
// with an alias.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "okteto"}
	root.AddCommand(&cobra.Command{Use: "up"})
	root.AddCommand(&cobra.Command{Use: "context", Aliases: []string{"ctx"}})
	root.AddCommand(&cobra.Command{Use: "namespace", Aliases: []string{"ns"}})
	root.AddCommand(&cobra.Command{Use: "restart", Hidden: true})
	root.AddCommand(&cobra.Command{Use: "generateFigSpec", Aliases: []string{"genFigSpec"}, Hidden: true})
	return root
}

// mustNotBeCalledLookPath returns a lookPathFn that fails the test if the
// resolver reaches any PATH lookup at all.
func mustNotBeCalledLookPath(t *testing.T) lookPathFn {
	t.Helper()
	return func(file string) (string, error) {
		t.Errorf("lookPath must not be called, got %q", file)
		return "", exec.ErrNotFound
	}
}

type lookupResult struct {
	path string
	err  error
}

// scriptedLookPath returns a lookPathFn that records every queried file into
// calls and returns the scripted results in call order.
func scriptedLookPath(calls *[]string, results ...lookupResult) lookPathFn {
	i := 0
	return func(file string) (string, error) {
		*calls = append(*calls, file)
		r := results[i]
		i++
		return r.path, r.err
	}
}

func TestResolveDispatchesFromPATH(t *testing.T) {
	tests := []struct {
		name           string
		expectedLookup string
		args           []string
	}{
		{
			name:           "unknown command",
			args:           []string{"okteto", "launch"},
			expectedLookup: "okteto-launch",
		},
		{
			name:           "unknown command with trailing flags",
			args:           []string{"okteto", "launch", "--port", "8080"},
			expectedLookup: "okteto-launch",
		},
		{
			name:           "dashed name with separator and flag-lookalikes",
			args:           []string{"okteto", "deploy-preview", "--", "-x"},
			expectedLookup: "okteto-deploy-preview",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			lookPath := scriptedLookPath(&calls, lookupResult{"/usr/local/bin/" + tt.expectedLookup, nil})

			path, ok := resolve(newTestRoot(), tt.args, lookPath, t.TempDir())

			require.True(t, ok)
			require.Equal(t, []string{tt.expectedLookup}, calls)
			require.Equal(t, "/usr/local/bin/"+tt.expectedLookup, path)
		})
	}
}

func TestResolveDispatchesFromPluginDir(t *testing.T) {
	pluginDir := t.TempDir()
	dirBinary := filepath.Join(pluginDir, "okteto-launch")
	var calls []string
	lookPath := scriptedLookPath(&calls,
		lookupResult{"", exec.ErrNotFound}, // not on PATH
		lookupResult{dirBinary, nil},       // found in the plugins dir
	)

	path, ok := resolve(newTestRoot(), []string{"okteto", "launch"}, lookPath, pluginDir)

	require.True(t, ok)
	require.Equal(t, dirBinary, path)
	require.Equal(t, []string{"okteto-launch", dirBinary}, calls)
}

func TestResolvePATHTakesPrecedenceOverPluginDir(t *testing.T) {
	var calls []string
	lookPath := scriptedLookPath(&calls, lookupResult{"/usr/local/bin/okteto-launch", nil})

	path, ok := resolve(newTestRoot(), []string{"okteto", "launch"}, lookPath, t.TempDir())

	require.True(t, ok)
	require.Equal(t, "/usr/local/bin/okteto-launch", path)
	require.Equal(t, []string{"okteto-launch"}, calls)
}

func TestResolveIgnoresNonAbsolutePluginDir(t *testing.T) {
	var calls []string
	lookPath := scriptedLookPath(&calls, lookupResult{"", exec.ErrNotFound})

	path, ok := resolve(newTestRoot(), []string{"okteto", "launch"}, lookPath, "relative/plugins")

	require.False(t, ok)
	require.Empty(t, path)
	require.Equal(t, []string{"okteto-launch"}, calls)
}

func TestResolveNoDispatchArgsShape(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "bare invocation",
			args: []string{"okteto"},
		},
		{
			name: "long flag as first arg",
			args: []string{"okteto", "--version"},
		},
		{
			name: "short flag before command token",
			args: []string{"okteto", "-l", "debug", "launch"},
		},
		{
			name: "empty token",
			args: []string{"okteto", ""},
		},
		{
			name: "token with unix path separator",
			args: []string{"okteto", "tools/build"},
		},
		{
			name: "token with windows path separator",
			args: []string{"okteto", `..\evil`},
		},
		{
			name: "token with windows volume separator",
			args: []string{"okteto", "C:evil"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := resolve(newTestRoot(), tt.args, mustNotBeCalledLookPath(t), t.TempDir())

			require.False(t, ok)
			require.Empty(t, path)
		})
	}
}

func TestResolveNoDispatchBuiltins(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "visible command", args: []string{"okteto", "up"}},
		{name: "aliased command by name", args: []string{"okteto", "context"}},
		{name: "aliased command by alias", args: []string{"okteto", "ctx"}},
		{name: "namespace alias", args: []string{"okteto", "ns"}},
		{name: "hidden command", args: []string{"okteto", "restart"}},
		{name: "hidden command by name", args: []string{"okteto", "generateFigSpec"}},
		{name: "hidden command by alias", args: []string{"okteto", "genFigSpec"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := resolve(newTestRoot(), tt.args, mustNotBeCalledLookPath(t), t.TempDir())

			require.False(t, ok)
			require.Empty(t, path)
		})
	}
}

func TestResolveNoDispatchReservedCobraNames(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "help", args: []string{"okteto", "help"}},
		{name: "completion", args: []string{"okteto", "completion", "bash"}},
		{name: "shell completion request", args: []string{"okteto", cobra.ShellCompRequestCmd, "lau"}},
		{name: "shell completion no-desc request", args: []string{"okteto", cobra.ShellCompNoDescRequestCmd, "lau"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := resolve(newTestRoot(), tt.args, mustNotBeCalledLookPath(t), t.TempDir())

			require.False(t, ok)
			require.Empty(t, path)
		})
	}
}

func TestResolveNoDispatchNotFoundAnywhere(t *testing.T) {
	tests := []struct {
		err  error
		name string
		path string
	}{
		{
			name: "not found in PATH nor plugin dir",
			path: "",
			err:  exec.ErrNotFound,
		},
		{
			name: "match relative to current directory is rejected",
			path: "./okteto-launch",
			err:  fmt.Errorf("okteto-launch: %w", exec.ErrDot),
		},
		{
			name: "permission denied",
			path: "",
			err:  fmt.Errorf("okteto-launch: %w", fs.ErrPermission),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			lookPath := scriptedLookPath(&calls, lookupResult{tt.path, tt.err}, lookupResult{tt.path, tt.err})

			path, ok := resolve(newTestRoot(), []string{"okteto", "launch"}, lookPath, t.TempDir())

			require.False(t, ok)
			require.Empty(t, path)
			require.Len(t, calls, 2)
		})
	}
}

func TestResolveNoDispatchSubcommandArgs(t *testing.T) {
	path, ok := resolve(newTestRoot(), []string{"okteto", "context", "unknownsub"}, mustNotBeCalledLookPath(t), t.TempDir())

	require.False(t, ok)
	require.Empty(t, path)
}

func TestPluginArgv(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		args     []string
		expected []string
	}{
		{
			name:     "no remaining args",
			path:     "/usr/local/bin/okteto-launch",
			args:     []string{"okteto", "launch"},
			expected: []string{"/usr/local/bin/okteto-launch"},
		},
		{
			name:     "flags separators and flag-lookalikes preserved verbatim",
			path:     "/usr/local/bin/okteto-launch",
			args:     []string{"okteto", "launch", "--port", "8080", "--", "-x", "positional"},
			expected: []string{"/usr/local/bin/okteto-launch", "--port", "8080", "--", "-x", "positional"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argv := pluginArgv(tt.path, tt.args)

			require.Equal(t, tt.expected, argv)
		})
	}
}
