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

package divert

import (
	"context"

	"github.com/okteto/okteto/pkg/divert/swap"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// downFlags are the values `okteto divert down` reads from the command line.
type downFlags struct {
	service string
	from    string
	key     string
	all     bool
}

// Down returns the `okteto divert down` command.
func Down(ctx context.Context, ioCtrl *io.Controller) *cobra.Command {
	flags := &downFlags{}

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop diverting a service and put the shared namespace back as it was",
		Long: `Stop diverting a service and put the shared namespace back as it was.

By default this removes only your own routing key. A service can be diverted by several
developers at once, and the router and the baseline come down only with the last routing
key, so leaving does not take anyone else's divert with it. Use --all to remove the whole
divert regardless of who else is using it.

Everything needed to undo a divert is stored on the service itself, so this works from any
machine with cluster access, whatever state the divert was left in. Running it against a
service that is not diverted succeeds without changing anything.`,
		Example: `  okteto divert down --service api --from staging
  okteto divert down --service api --from staging --all`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := setupClient(ctx, ioCtrl)
			if err != nil {
				return err
			}

			opts := swap.DownOptions{
				Service:         flags.service,
				SharedNamespace: flags.from,
				RoutingKey:      downRoutingKey(flags),
				All:             flags.all,
			}

			oktetoLog.Spinner("Restoring service " + flags.service + "...")
			oktetoLog.StartSpinner()
			defer oktetoLog.StopSpinner()

			if err := client.Down(ctx, opts); err != nil {
				return err
			}

			oktetoLog.StopSpinner()
			oktetoLog.Success("Service '%s/%s' is no longer diverted", opts.SharedNamespace, opts.Service)

			return nil
		},
	}

	cmd.Flags().StringVar(&flags.service, "service", "", "name of the service to stop diverting (mandatory)")
	cmd.Flags().StringVar(&flags.from, "from", "", "shared namespace holding the diverted service (mandatory)")
	cmd.Flags().StringVar(&flags.key, "key", "", "routing key to stop diverting (defaults to the current namespace)")
	cmd.Flags().BoolVar(&flags.all, "all", false, "remove the whole divert, including other developers' routing keys")

	cmd.MarkFlagRequired("service")
	cmd.MarkFlagRequired("from")

	return cmd
}

// downRoutingKey defaults to the current namespace, the same default `up` uses, so that
// leaving a divert removes your own route rather than everybody's.
func downRoutingKey(flags *downFlags) string {
	if flags.all {
		return ""
	}

	return routingKeyOrDefault(flags.key, okteto.GetContext().Namespace)
}
