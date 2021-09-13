// Copyright 2021 The Okteto Authors
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
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	okContext "github.com/okteto/okteto/pkg/cmd/context"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

type ContextOptions struct {
	Token           string
	isOktetoCluster bool
}

// Context points okteto to a cluster.
func Context() *cobra.Command {
	ctxOptions := &ContextOptions{}
	cmd := &cobra.Command{
		Use:     "context [url|k8s-context]",
		Aliases: []string{"ctx"},
		Args:    utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#context"),
		Short:   "Points okteto to a cluster",
		Long: `Points okteto to a cluster

Run
    $ okteto context

and this command will ask you for the cluster you want to operate in okteto.
If an okteto cluster is selected it will open your browser to ask your authentication details and retrieve your API token. You can script it by using the --token parameter.

By default, this will ask you for the cluster you want to operate. 

If you want to log into your Okteto Enterprise instance, specify a URL. For example, run

    $ okteto context https://okteto.example.com

to log in to a Okteto Enterprise instance running at okteto.example.com and point okteto to it.

If you want to log into your own cluster instance, specify a kubernetes context. For example, run

    $ okteto context my_cluster

to point okteto to 'mycluster'.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if ctxOptions.Token == "" && k8Client.InCluster() {
				return fmt.Errorf("this command is not supported without the '--token' flag from inside a pod")
			}

			var err error
			if len(args) == 0 {
				log.Infof("authenticating without context")
				err = runInteractiveContext(ctx, ctxOptions)

			} else {
				log.Infof("authenticating with context arg")
				err = runContextWithArgs(ctx, args[0], ctxOptions)
			}

			if err != nil {
				analytics.TrackContextLogin(false, ctxOptions.isOktetoCluster)
				analytics.TrackLogin(false, "", "", "", "")
				return err
			}

			analytics.TrackContextLogin(true, ctxOptions.isOktetoCluster)
			log.Success("Your context have been updated")
			return nil
		},
	}

	cmd.Flags().StringVarP(&ctxOptions.Token, "token", "t", "", "API token for authentication.")
	return cmd
}

func runInteractiveContext(ctx context.Context, ctxOptions *ContextOptions) error {

	cluster, err := getCluster(ctx, ctxOptions)
	if err != nil {
		return err
	}

	err = saveOktetoContext(ctx, cluster)
	if err != nil {
		return err
	}
	return nil
}

func runContextWithArgs(ctx context.Context, cluster string, ctxOptions *ContextOptions) error {

	if isURL(cluster) {
		ctxOptions.isOktetoCluster = true
		err := authenticateToOktetoCluster(ctx, cluster, ctxOptions.Token)
		if err != nil {
			return err
		}
	} else {
		ctxOptions.isOktetoCluster = false

		if !isValidCluster(cluster) {
			return fmt.Errorf("'%s' is a invalid cluster. Select one from ['%s']", cluster, strings.Join(getClusterList(), "', '"))
		}
		err := okContext.CopyK8sClusterConfigToOktetoContext(cluster)
		if err != nil {
			return err
		}
	}

	err := saveOktetoContext(ctx, cluster)
	if err != nil {
		return err
	}

	return nil
}
