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

	"github.com/spf13/cobra"
)

const (
	commandName = "git-credential"
	actionGet   = "get"
	actionStore = "store"
	actionErase = "erase"
)

// GitCredential is the Git credential helper for repositories on github.com.
// Git runs it as "<helper> get|store|erase"; see https://git-scm.com/docs/gitcredentials
func GitCredential(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Git credential helper for GitHub repositories connected to Okteto",
		Long: `Acts as a Git credential helper that asks Okteto for read-only credentials to GitHub repositories:

Usage: printf 'protocol=https\nhost=github.com\npath=owner/repo.git\n\n' | okteto git-credential get

It sends OKTETO_TOKEN to the Okteto instance in OKTETO_URL (or OKTETO_CONTEXT).
"store" and "erase" do nothing.

More info about Git credential helpers here: https://git-scm.com/docs/gitcredentials
`,
		Hidden: true,
	}

	cmd.AddCommand(getCmd(ctx))
	cmd.AddCommand(noopCmd(actionStore))
	cmd.AddCommand(noopCmd(actionErase))
	cmd.AddCommand(Install())
	return cmd
}

func getCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   actionGet,
		Short: "Print the credentials for the repository in Git's request",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			get(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), newGetterFromEnv)
		},
	}
}

// noopCmd handles the actions Okteto ignores: credentials are never stored
func noopCmd(action string) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: "Do nothing: Okteto doesn't store Git credentials",
		Args:  cobra.NoArgs,
		Run:   func(*cobra.Command, []string) {},
	}
}

// IsGitCredentialHelperCommand returns true when Git runs the helper, so main
// can keep stdout free of anything but the credentials. Flags may come before the
// action: Git appends it to the helper, so "!okteto git-credential -l debug" runs
// "okteto git-credential -l debug get".
func IsGitCredentialHelperCommand(args []string) bool {
	if !IsGitCredentialCommand(args) {
		return false
	}
	for _, arg := range args[2:] {
		switch arg {
		case actionGet, actionStore, actionErase:
			return true
		}
	}
	return false
}

// IsGitCredentialCommand returns true for any git-credential command, so main
// skips what these commands don't need, such as analytics
func IsGitCredentialCommand(args []string) bool {
	return len(args) > 1 && args[1] == commandName
}
