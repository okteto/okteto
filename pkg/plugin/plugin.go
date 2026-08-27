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

// Package plugin implements an alpha kubectl/git-style plugin passthrough:
// when enabled, an unknown top-level command like `okteto launch` is
// forwarded to a binary named `okteto-launch` found in PATH or in the okteto
// plugins directory (<okteto-home>/plugins). Builtin commands always take
// precedence, and there are no plugin management commands. Plugin binaries
// run with the user's privileges and full environment, and no signature or
// digest verification is performed — the same trust model used by kubectl
// plugins and git external subcommands.
//
// The passthrough is only available on unix during this alpha: MaybeExec is
// a no-op on Windows (see plugin_windows.go).
package plugin

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// OktetoAlphaPluginEnabledEnvVar enables the alpha plugin passthrough.
// Default is false (unset): brand-new, unvalidated feature — keeping it off
// preserves today's behavior exactly, so unknown commands keep failing with
// cobra's "unknown command" error.
const OktetoAlphaPluginEnabledEnvVar = "OKTETO_ALPHA_PLUGIN_ENABLED"

// binaryPrefix is hardcoded instead of derived from os.Args[0] so that
// renaming or symlinking the okteto binary cannot change which plugin
// binaries are resolved.
const binaryPrefix = "okteto-"

// reservedNames are cobra commands registered lazily inside Execute()
// (initCompleteCmd is unexported in cobra), so a pre-Execute root.Find
// cannot see them; this skip-list is the only way to guarantee they are
// never shadowed by a plugin binary.
var reservedNames = map[string]struct{}{
	"help":                          {},
	"completion":                    {},
	cobra.ShellCompRequestCmd:       {},
	cobra.ShellCompNoDescRequestCmd: {},
}

type lookPathFn func(file string) (string, error)

// resolve decides whether args must be dispatched to a plugin: args[1]
// shaped like a command token (non-empty, no leading dash, no path
// separator), not a reserved cobra name, not a builtin (root.Find matches
// names, aliases, and hidden commands), and a matching okteto-<name> binary
// found in PATH or, failing that, in pluginDir. Both lookups go through
// lookPath (exec.LookPath), so a match relative to the current directory
// (exec.ErrDot) is rejected; pluginDir is used only when absolute, so its
// candidate path is never cwd-relative. Any lookup failure means "no
// plugin", so cobra keeps producing its usual unknown-command error.
func resolve(root *cobra.Command, args []string, lookPath lookPathFn, pluginDir string) (string, bool) {
	if len(args) <= 1 {
		return "", false
	}
	name := args[1]
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, `/\:`) {
		// A name with a path separator makes exec.LookPath resolve it
		// directly instead of searching PATH, which bypasses the ErrDot
		// rejection below and would run a binary relative to the current
		// directory. Command tokens never contain separators, so reject them.
		return "", false
	}
	if _, isReserved := reservedNames[name]; isReserved {
		return "", false
	}
	if _, _, err := root.Find([]string{name}); err == nil {
		return "", false
	}

	binary := binaryPrefix + name
	if path, err := lookPath(binary); err == nil {
		return path, true
	}
	if filepath.IsAbs(pluginDir) {
		if path, err := lookPath(filepath.Join(pluginDir, binary)); err == nil {
			return path, true
		}
	}
	return "", false
}

// pluginArgv builds the child argv: the resolved path as argv[0], then the
// invocation's remaining arguments verbatim.
func pluginArgv(path string, args []string) []string {
	return append([]string{path}, args[2:]...)
}
