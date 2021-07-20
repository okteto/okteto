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

package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func destroy(ctx context.Context) *cobra.Command {
	var name string
	var namespace string
	var wait bool
	var destroyVolumes bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroys an okteto pipeline",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#destroy"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			if !okteto.IsAuthenticated() {
				return errors.ErrNotLogged
			}

			var err error
			if name == "" {
				name, err = getPipelineName()
				if err != nil {
					return err
				}
			}

			if namespace == "" {
				namespace = getCurrentNamespace(ctx)
			}

			currentContext := client.GetSessionContext("")
			if okteto.GetClusterContext() != currentContext {
				log.Information("Pipeline context: %s/%s", okteto.GetURL(), namespace)
			}

			if err := deletePipeline(ctx, name, namespace, wait, destroyVolumes, timeout); err != nil {
				return err
			}

			if wait {
				log.Success("Pipeline '%s' destroyed", name)
			} else {
				log.Success("Pipeline '%s' scheduled for destruction", name)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "p", "", "name of the pipeline (defaults to the folder name)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().BoolVarP(&destroyVolumes, "volumes", "v", false, "destroy persistent volumes created by the pipeline (defaults to false)")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	return cmd
}

func deletePipeline(ctx context.Context, name, namespace string, wait, destroyVolumes bool, timeout time.Duration) error {
	spinner := utils.NewSpinner("Destroying your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	_, err := okteto.DeletePipeline(ctx, name, namespace, destroyVolumes)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Infof("pipeline '%s' not found", name)
			return nil
		}

		return fmt.Errorf("failed to delete pipeline '%s': %w", name, err)
	}

	if !wait {
		return nil
	}

	// this will also run if it's not found
	return waitUntilRunning(ctx, name, namespace, timeout)
}
