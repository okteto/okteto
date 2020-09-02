// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Destroy an okteto pipeline
func Destroy(ctx context.Context) *cobra.Command {
	var name string
	var namespace string
	var wait bool

	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Deletes an okteto pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			var err error
			if name == "" {
				name, err = getPipelineName()
				if err != nil {
					return err
				}
			}

			if namespace == "" {
				namespace, err = getCurrentNamespace(ctx)
				if err != nil {
					return err
				}
			}

			if err := deletePipeline(ctx, name, namespace, wait); err != nil {
				return err
			}

			log.Success("Pipeline '%s' deleted", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "p", "", "name of the pipeline (defaults to the folder name)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	return cmd
}

func deletePipeline(ctx context.Context, name, namespace string, wait bool) error {
	spinner := utils.NewSpinner("Deleting your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	_, err := okteto.DeletePipeline(ctx, name, namespace)
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
	return waitUntilRunning(ctx, name, namespace)
}
