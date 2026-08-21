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
// forwarded to a binary named `okteto-launch` found in PATH. Builtin
// commands always take precedence, and there are no plugin management
// commands. Plugin binaries run with the user's privileges and full
// environment, and no signature or digest verification is performed — the
// same trust model used by kubectl plugins and git external subcommands.
package plugin

import (
	"os"
	"os/exec"
	"strings"

	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
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

// MaybeExec forwards the invocation to an okteto-<name> plugin binary from
// PATH when OKTETO_ALPHA_PLUGIN_ENABLED is true and os.Args[1] is not a
// builtin command. It returns whenever cobra should handle the invocation
// instead. When a plugin is dispatched it never returns: the process image
// is replaced (unix) or the process exits with the child's exit code
// (windows), or with 1 if the resolved binary fails to start.
func MaybeExec(root *cobra.Command) {
	path, ok := resolve(root, os.Args, env.LoadBoolean(OktetoAlphaPluginEnabledEnvVar), exec.LookPath)
	if !ok {
		return
	}
	oktetoLog.Infof("dispatching %q to plugin binary %s", os.Args[1], path)
	if err := execPlugin(path, pluginArgv(path, os.Args), os.Environ()); err != nil {
		oktetoLog.Fail("failed to execute plugin %s: %s", path, err)
		os.Exit(1) // skipcq: RVV-A0003
	}
}

// resolve decides whether args must be dispatched to a plugin: gate on,
// args[1] shaped like a command token (non-empty, no leading dash, no path
// separator), not a reserved cobra name, not a builtin (root.Find matches
// names, aliases, and hidden commands), and a matching okteto-<name> binary
// found in PATH. Any lookPath failure — including exec.ErrDot for matches
// relative to the current directory, which are deliberately rejected —
// means "no plugin", so cobra keeps producing its usual unknown-command
// error.
func resolve(root *cobra.Command, args []string, enabled bool, lookPath lookPathFn) (string, bool) {
	if !enabled || len(args) <= 1 {
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
	path, err := lookPath(binaryPrefix + name)
	if err != nil {
		return "", false
	}
	return path, true
}

// pluginArgv builds the child argv: the resolved path as argv[0], then the
// invocation's remaining arguments verbatim.
func pluginArgv(path string, args []string) []string {
	return append([]string{path}, args[2:]...)
}
