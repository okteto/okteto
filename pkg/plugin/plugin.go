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
// when enabled, an unknown top-level command like `okteto launch` execs
// `okteto-launch` from PATH or <okteto-home>/plugins. Builtins always win.
// Plugins run with the user's privileges and full environment, with no
// signature or digest verification — the kubectl/git trust model. Unix-only
// during the alpha: MaybeExec is a no-op on Windows.
package plugin

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// OktetoAlphaPluginEnabledEnvVar enables the alpha plugin passthrough;
// default off (unset) preserves today's unknown-command behavior exactly.
const OktetoAlphaPluginEnabledEnvVar = "OKTETO_ALPHA_PLUGIN_ENABLED"

// Not derived from os.Args[0]: renaming/symlinking okteto must not change resolution.
const binaryPrefix = "okteto-"

// Cobra registers these lazily inside Execute(), so a pre-Execute root.Find
// can't see them; skip-listed so a plugin can never shadow them.
var reservedNames = map[string]struct{}{
	"help":                          {},
	"completion":                    {},
	cobra.ShellCompRequestCmd:       {},
	cobra.ShellCompNoDescRequestCmd: {},
}

type lookPathFn func(file string) (string, error)

// resolve reports whether args[1] must be dispatched to a plugin:
// command-shaped token, not reserved, not a builtin (root.Find matches names,
// aliases, hidden commands), and okteto-<name> resolved via lookPath to an
// absolute path in PATH, then in pluginDir (absolute only). cwd-relative
// matches and all lookup failures mean "no plugin" — cobra's error stands.
func resolve(root *cobra.Command, args []string, lookPath lookPathFn, pluginDir string) (string, bool) {
	if len(args) <= 1 {
		return "", false
	}
	name := args[1]
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, `/\:`) {
		// A separator makes exec.LookPath resolve the name directly instead of
		// searching PATH, bypassing the ErrDot rejection — i.e. run from cwd.
		return "", false
	}
	if _, isReserved := reservedNames[name]; isReserved {
		return "", false
	}
	if _, _, err := root.Find([]string{name}); err == nil {
		return "", false
	}

	binary := binaryPrefix + name
	// Require an absolute result so a cwd-relative match is rejected even under
	// GODEBUG=execerrdot=0.
	if path, err := lookPath(binary); err == nil && filepath.IsAbs(path) {
		return path, true
	}
	if filepath.IsAbs(pluginDir) {
		if path, err := lookPath(filepath.Join(pluginDir, binary)); err == nil {
			return path, true
		}
	}
	return "", false
}

// pluginArgv: the resolved path as argv[0], then the remaining args verbatim.
func pluginArgv(path string, args []string) []string {
	return append([]string{path}, args[2:]...)
}
