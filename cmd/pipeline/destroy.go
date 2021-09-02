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

			pipeline, err := destroyPipeline(ctx, name, namespace, destroyVolumes)
			if err != nil {
				return err
			}

			if !wait {
				log.Success("Pipeline '%s' scheduled for destruction", name)
				return nil
			}

			if err := waitUntilDestroyed(ctx, name, namespace, timeout); err != nil {
				log.Information("Pipeline URL: %s", getPipelineURL(namespace, pipeline))
				return err
			}

			log.Success("Pipeline '%s' successfully destroyed", name)

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

func destroyPipeline(ctx context.Context, name, namespace string, destroyVolumes bool) (*okteto.PipelineRun, error) {
	spinner := utils.NewSpinner("Destroying your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var err error
	var pipeline *okteto.PipelineRun
	go func() {

		pipeline, err = okteto.DestroyPipeline(ctx, name, namespace, destroyVolumes)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Infof("pipeline '%s' not found", name)
				exit <- nil
			}
			exit <- fmt.Errorf("failed to destroy pipeline '%s': %w", name, err)
		}
		exit <- nil
	}()
	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return nil, err
		}
	}
	return pipeline, nil
}

func waitUntilDestroyed(ctx context.Context, name, namespace string, timeout time.Duration) error {
	spinner := utils.NewSpinner("Waiting for the pipeline to be destroyed...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		exit <- waitToBeDestroyed(ctx, name, namespace, timeout)
	}()

	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		os.Exit(130)
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return err
		}
	}

	return nil
}

func waitToBeDestroyed(ctx context.Context, name, namespace string, timeout time.Duration) error {

	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("pipeline '%s' didn't finish after %s", name, timeout.String())
		case <-t.C:
			p, err := okteto.GetPipelineByName(ctx, name, namespace)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsNotExist(err) {
					return nil
				}
				return fmt.Errorf("failed to get pipeline '%s': %s", name, err)
			}

			if p.Status == "error" {
				return fmt.Errorf("pipeline '%s' failed", name)
			}
		}
	}
}
