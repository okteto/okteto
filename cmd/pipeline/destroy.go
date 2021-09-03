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
	"os"
	"os/signal"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
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

			if name == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get the current working directory: %w", err)
				}
				repo, err := model.GetRepositoryURL(cwd)
				if err != nil {
					return err
				}

				name = getPipelineName(repo)
			}

			if namespace == "" {
				namespace = getCurrentNamespace(ctx)
			}

			if err := deletePipeline(ctx, name, namespace, destroyVolumes); err != nil {
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

	cmd.Flags().StringVarP(&name, "name", "p", "", "name of the pipeline (defaults to the git config name)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().BoolVarP(&destroyVolumes, "volumes", "v", false, "destroy persistent volumes created by the pipeline (defaults to false)")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	return cmd
}

func deletePipeline(ctx context.Context, name, namespace string, destroyVolumes bool) error {
	spinner := utils.NewSpinner("Destroying your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		_, err := okteto.DeletePipeline(ctx, name, namespace, destroyVolumes)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Infof("pipeline '%s' not found", name)
				exit <- nil
			}

			exit <- fmt.Errorf("failed to delete pipeline '%s': %w", name, err)
		}
	}()
	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}
