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

package cmd

import (
	"context"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Login starts the login handshake with GitHub and okteto
func Login() *cobra.Command {
	token := ""
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "login [url]",
		Args:   utils.MaximumNArgsAccepted(1, "https://www.okteto.com/docs/reference/okteto-cli/#context"),
		Short:  "Log into Okteto",
		Long: `Log into Okteto

Run
    $ okteto login

and this command will open your browser to ask your authentication details and retrieve your API token. You can script it by using the --token parameter.

By default, this will log into cloud.okteto.com. If you want to log into your Okteto Enterprise instance, specify a URL. For example, run

    $ okteto login https://okteto.example.com

to log in to a Okteto Enterprise instance running at okteto.example.com.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning(`'okteto login' is deprecated in favor of 'okteto context', and will be removed in a future version.
    Learn more about okteto context at https://okteto.com/docs/reference/okteto-cli/#context`)

			ctxOptions := contextCMD.Options{
				IsCtxCommand: true,
				IsOkteto:     true,
				Save:         true,
				Token:        token,
			}
			if len(args) == 1 {
				args[0] = okteto.AddSchema(args[0])
				args[0] = strings.TrimSuffix(args[0], "/")
				ctxOptions.Context = args[0]
			}

			ctx := context.Background()
			err := contextCMD.NewContextCommand().Run(ctx, &ctxOptions)
			if err != nil {
				analytics.TrackLogin(false)
			} else {
				analytics.TrackLogin(true)
			}
			return err

		},
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "API token for authentication.  (optional)")
	return cmd
}
