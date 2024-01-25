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
func Context(okClientProvider oktetoClientProvider) *cobra.Command {
	ctxOptions := &Options{}
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Args:    utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#context"),
		Short:   "Set the default context",
		Long: `Set the default context

A context is a group of cluster access parameters. Each context contains a Kubernetes cluster, a user, and a namespace.
The current context is the default cluster/namespace for any Okteto CLI command.

To set your default context, run the ` + "`okteto context`" + ` command:

    $ okteto context

This will prompt you to select one of your existing contexts or to create a new one.
`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// read parent PersistentPreRun if any and run them before current PersistentPreRun
			if parent := cmd.Parent(); parent != nil {
				if parent.PersistentPreRun != nil {
					parent.PersistentPreRun(parent, args)
				}
			}
			okteto.SetInsecureSkipTLSVerifyPolicy(ctxOptions.InsecureSkipTlsVerify)
		},
		RunE: Use().RunE,
	}
	cmd.AddCommand(Show())
	cmd.AddCommand(Use())
	cmd.AddCommand(List())
	cmd.AddCommand(DeleteCMD())

	// deprecated
	cmd.AddCommand(CreateCMD())
	cmd.AddCommand(UpdateKubeconfigCMD(okClientProvider))
	cmd.AddCommand(UseNamespace())

	cmd.PersistentFlags().BoolVarP(&ctxOptions.InsecureSkipTlsVerify, "insecure-skip-tls-verify", "", false, " If enabled, the server's certificate will not be checked for validity. This will make your connections insecure")
	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication")
	cmd.Flags().StringVarP(&ctxOptions.Namespace, "namespace", "n", "", "namespace of your okteto context")
	cmd.Flags().StringVarP(&ctxOptions.Builder, "builder", "b", "", "url of the builder service")
	cmd.Flags().BoolVarP(&ctxOptions.OnlyOkteto, "okteto", "", false, "only shows okteto context options")
	if err := cmd.Flags().MarkHidden("okteto"); err != nil {
		oktetoLog.Infof("failed to mark 'okteto' flag as hidden: %s", err)
	}
	return cmd
}
