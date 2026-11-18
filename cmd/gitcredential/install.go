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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

const (
	helperKey      = "credential.https://github.com.helper"
	useHTTPPathKey = "credential.https://github.com.useHttpPath"

	globalFlag = "global"
	systemFlag = "system"
)

var safeShellWord = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

type installOptions struct {
	global bool
	system bool
}

// gitRunner runs git with the given args and returns its stdout
type gitRunner func(ctx context.Context, args ...string) (string, error)

// Install writes the helper into the global or system Git config, for https://github.com only
func Install() *cobra.Command {
	opts := &installOptions{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Okteto's Git credential helper for github.com",
		Long: `Install Okteto's Git credential helper for https://github.com in the global or system Git config.

It replaces the other credential helpers for https://github.com at that scope, as "gh auth setup-git" does,
and sets useHttpPath so Git sends the repository in its requests. With --global, the empty helper it writes
also drops the helpers that the system config sets for github.com, such as osxkeychain or Git Credential
Manager, and useHttpPath makes "store" and "cache" skip the github.com credentials they saved without a path.
There is no uninstall: remove the credential "https://github.com" section from the Git config by hand.

It does nothing when OKTETO_DISABLE_GIT_CREDENTIAL_HELPER is true.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if isDisabled() {
				oktetoLog.Information("Okteto's Git credential helper is turned off by %s, skipping", constants.OktetoDisableGitCredentialHelperEnvVar)
				return nil
			}

			scope := globalFlag
			if opts.system {
				scope = systemFlag
			}

			gitPath, err := exec.LookPath("git")
			if err != nil {
				oktetoLog.Warning("git is not installed, skipping Okteto's Git credential helper")
				return nil
			}

			okteto, err := os.Executable()
			if err != nil {
				return fmt.Errorf("couldn't find the okteto binary: %w", err)
			}

			if err := install(cmd.Context(), execGit(gitPath), scope, helperCommand(okteto)); err != nil {
				return err
			}
			oktetoLog.Success("Okteto's Git credential helper successfully installed in the %s Git config", scope)
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.global, globalFlag, false, "write to the global Git config file of the current user")
	cmd.Flags().BoolVar(&opts.system, systemFlag, false, "write to the system Git config file")
	cmd.MarkFlagsMutuallyExclusive(globalFlag, systemFlag)
	cmd.MarkFlagsOneRequired(globalFlag, systemFlag)
	return cmd
}

// install clears the helpers configured for https://github.com at the scope before
// adding Okteto's, as "gh auth setup-git" does: the empty value resets the list of
// helpers Git has read so far, so Git never asks another helper first. Running it
// again leaves the same config.
func install(ctx context.Context, git gitRunner, scope, helper string) error {
	if scope == systemFlag {
		ensureSystemConfigDir(ctx, git)
	}

	scopeArg := fmt.Sprintf("--%s", scope)
	steps := [][]string{
		{"config", scopeArg, "--replace-all", helperKey, ""},
		{"config", scopeArg, "--add", helperKey, helper},
		{"config", scopeArg, useHTTPPathKey, "true"},
	}
	for _, args := range steps {
		if _, err := git(ctx, args...); err != nil {
			return err
		}
	}
	return nil
}

// ensureSystemConfigDir creates the directory of the system Git config when it is missing:
// Git built from source with prefix /usr, as in the Okteto CLI image, reads /usr/etc/gitconfig
// and can't create it without /usr/etc. "git var GIT_CONFIG_SYSTEM" needs Git 2.42; with an
// older Git this does nothing and git config reports the error.
func ensureSystemConfigDir(ctx context.Context, git gitRunner) {
	out, err := git(ctx, "var", "GIT_CONFIG_SYSTEM")
	if err != nil {
		oktetoLog.Infof("couldn't find the system Git config: %s", err)
		return
	}
	path := strings.TrimSpace(out)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		oktetoLog.Infof("couldn't create the directory of the system Git config %q: %s", path, err)
	}
}

// execGit runs the git binary at gitPath
func execGit(gitPath string) gitRunner {
	return func(ctx context.Context, args ...string) (string, error) {
		var stdout, stderr bytes.Buffer
		c := exec.CommandContext(ctx, gitPath, args...)
		c.Stdout = &stdout
		c.Stderr = &stderr
		if err := c.Run(); err != nil {
			return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), nil
	}
}

// helperCommand runs the okteto binary at its absolute path through the shell,
// so the helper doesn't depend on the PATH of whatever process runs Git
func helperCommand(okteto string) string {
	return fmt.Sprintf("!%s %s", shellQuote(okteto), commandName)
}

func shellQuote(s string) string {
	if safeShellWord.MatchString(s) {
		return s
	}
	return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", `'\''`))
}
