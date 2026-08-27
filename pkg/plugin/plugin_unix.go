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

//go:build !windows
// +build !windows

package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

// pluginsDir is the subdirectory of the okteto home that holds plugin
// binaries (<okteto-home>/plugins).
const pluginsDir = "plugins"

// MaybeExec forwards the invocation to an okteto-<name> plugin binary from
// PATH or the okteto plugins directory when OKTETO_ALPHA_PLUGIN_ENABLED is
// true and os.Args[1] is not a builtin command. It returns whenever cobra
// should handle the invocation instead. When a plugin is dispatched it never
// returns: syscall.Exec replaces the current process image with the plugin
// binary — the mechanism kubectl and git use, so the plugin owns the TTY,
// signals, and the exit code — or, if the resolved binary fails to start,
// the process exits with status 1.
func MaybeExec(root *cobra.Command) {
	if !env.LoadBoolean(OktetoAlphaPluginEnabledEnvVar) {
		// Return before resolving the okteto home directory so a disabled
		// feature has no side effects (GetOktetoHome creates the folder).
		return
	}
	dir := filepath.Join(config.GetOktetoHome(), pluginsDir)
	path, ok := resolve(root, os.Args, exec.LookPath, dir)
	if !ok {
		return
	}
	oktetoLog.Infof("dispatching %q to plugin binary %s", os.Args[1], path)
	if err := syscall.Exec(path, pluginArgv(path, os.Args), os.Environ()); err != nil {
		oktetoLog.Fail("failed to execute plugin %s: %s", path, err)
		os.Exit(1) // skipcq: RVV-A0003
	}
}
