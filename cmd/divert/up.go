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
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/divert/swap"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// upFlags are the values `okteto divert up` reads from the command line.
type upFlags struct {
	service          string
	from             string
	to               string
	key              string
	readinessTimeout time.Duration
	noRestart        bool
}

// Up returns the `okteto divert up` command.
func Up(ctx context.Context, ioCtrl *io.Controller) *cobra.Command {
	flags := &upFlags{}

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Divert a service from a shared namespace to your own",
		Long: `Divert a service from a shared namespace to your own.

A router is placed in front of the service in the shared namespace. Requests carrying
'baggage: divert=<key>' reach your copy of that service; every other request, and every
other service in the call chain, keeps hitting the shared one.

Your application must forward the baggage header across its own outbound calls for hops
past the first one to be diverted.

Once the router is serving, the diverted workload is restarted. Callers pool their
connections, and a connection opened before the divert keeps reaching the shared version
no matter what header it carries; restarting the workload those connections terminate at
is what makes them reconnect through the router. Use --no-restart to skip it.`,
		Example: `  okteto divert up --service api --from staging --key alice
  curl -H 'baggage: divert=alice' http://frontend.staging/`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := setupClient(ctx, ioCtrl)
			if err != nil {
				return err
			}

			target := targetNamespace(flags.to)
			opts := swap.UpOptions{
				Service:         flags.service,
				SharedNamespace: flags.from,
				TargetNamespace: target,
				RoutingKey:      routingKeyOrDefault(flags.key, target),
				// The router is this same binary's hidden `divert-router` command, so the
				// image is the CLI's own, version-matched by construction.
				RouterImage:         config.NewImageConfig(ioCtrl.Logger()).GetCliImage(),
				ReadinessTimeout:    flags.readinessTimeout,
				SkipBaselineRestart: flags.noRestart,
			}

			oktetoLog.Spinner("Diverting service " + flags.service + "...")
			oktetoLog.StartSpinner()
			defer oktetoLog.StopSpinner()

			if err := client.Up(ctx, opts); err != nil {
				return err
			}

			oktetoLog.StopSpinner()
			oktetoLog.Success(
				"Service '%s/%s' diverted to '%s'. Send requests with the header 'baggage: divert=%s'",
				opts.SharedNamespace, opts.Service, opts.TargetNamespace, opts.RoutingKey,
			)

			return nil
		},
	}

	cmd.Flags().StringVar(&flags.service, "service", "", "name of the service to divert (mandatory)")
	cmd.Flags().StringVar(&flags.from, "from", "", "shared namespace holding the service to divert (mandatory)")
	cmd.Flags().StringVar(&flags.to, "to", "", "namespace to divert the service to (defaults to the current namespace)")
	cmd.Flags().StringVar(&flags.key, "key", "", "routing key that selects the diverted copy (defaults to the target namespace)")
	cmd.Flags().DurationVar(&flags.readinessTimeout, "timeout", 0, "how long to wait for the router before giving up")
	cmd.Flags().BoolVar(&flags.noRestart, "no-restart", false, "do not restart the diverted workload; callers holding an open connection will keep reaching the shared version until they reconnect")

	cmd.MarkFlagRequired("service")
	cmd.MarkFlagRequired("from")

	return cmd
}

// targetNamespace defaults to wherever the developer is already working.
func targetNamespace(to string) string {
	if to != "" {
		return to
	}

	return okteto.GetContext().Namespace
}

// routingKeyOrDefault falls back to the target namespace, which is already unique per
// developer and so makes a good routing key. It defaults to the target rather than to the
// current namespace so that --to and --key stay consistent when they are used together.
func routingKeyOrDefault(key, target string) string {
	if key != "" {
		return key
	}

	return target
}
