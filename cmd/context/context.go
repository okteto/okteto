// Copyright 2023 The Okteto Authors
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

package context

import (
	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Context points okteto to a cluster.
func Context() *cobra.Command {
	ctxOptions := &Options{}
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Args:    utils.NoArgsAccepted("https://okteto.com/docs/reference/okteto-cli/#context"),
		Short:   "Set the default Okteto Context",
		Long: `Set the default Okteto Context.

An Okteto Context is a group of cluster access parameters.
Each context contains a Kubernetes cluster, a user, and a namespace.
The current Okteto Context is the default cluster/namespace for any Okteto CLI command.

To set your default Okteto Context, run the ` + "`okteto context`" + ` command:

    $ okteto context

This will prompt you to select one of your existing Okteto Contexts or to create a new one.
`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			okteto.SetInsecureSkipTLSVerifyPolicy(ctxOptions.InsecureSkipTlsVerify)
		},
		RunE: Use().RunE,
	}
	cmd.AddCommand(Show())
	cmd.AddCommand(Use())
	cmd.AddCommand(List())
	cmd.AddCommand(DeleteCMD())

	cmd.PersistentFlags().BoolVarP(&ctxOptions.InsecureSkipTlsVerify, "insecure-skip-tls-verify", "", false, "skip validation of server's certificates")
	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication. Use this when scripting or if you don't want to use browser-based authentication")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "overwrite the current Okteto Namespace")
	cmd.Flags().BoolVarP(&ctxOptions.OnlyOkteto, "okteto", "", false, "only shows okteto context options")
	cmd.Flags().BoolVarP(&ctxOptions.NoBrowser, "no-browser", "", false, "disable automatically opening the verification URL in the default browser")
	if err := cmd.Flags().MarkHidden("okteto"); err != nil {
		oktetoLog.Infof("failed to mark 'okteto' flag as hidden: %s", err)
	}
	return cmd
}
