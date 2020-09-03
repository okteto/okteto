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
	"os"
	"path/filepath"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func deploy(ctx context.Context) *cobra.Command {
	var branch string
	var repository string
	var name string
	var namespace string
	var wait bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys an okteto pipeline",
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

			if repository == "" {
				// TODO: get local repository
				return fmt.Errorf("repository is missing")
			}

			if branch == "" {
				// TODO: get local branch
				return fmt.Errorf("branch is missing")
			}

			if namespace == "" {
				namespace, err = getCurrentNamespace(ctx)
				if err != nil {
					return err
				}
			}

			if err := deployPipeline(ctx, name, namespace, repository, branch, wait); err != nil {
				return err
			}

			log.Success("Pipeline '%s' created", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "p", "", "name of the pipeline (defaults to the folder name)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "the branch to deploy (defaults to the main branch)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	return cmd
}

func deployPipeline(ctx context.Context, name, namespace, repository, branch string, wait bool) error {
	spinner := utils.NewSpinner("Creating your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	_, err := okteto.DeployPipeline(ctx, name, namespace, repository, branch)
	if err != nil {
		return fmt.Errorf("failed to deploy pipeline: %w", err)
	}

	if !wait {
		return nil
	}

	spinner.Update("Waiting for the pipeline to finish...")
	return waitUntilRunning(ctx, name, namespace)
}

func getPipelineName() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Base(workDir), nil
}

func waitUntilRunning(ctx context.Context, name, namespace string) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-to.C:
			return fmt.Errorf("pipeline '%s' didn't finish after 5 minutes", name)
		case <-t.C:
			p, err := okteto.GetPipelineByName(ctx, name, namespace)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsNotExist(err) {
					return nil
				}

				return fmt.Errorf("failed to get pipeline '%s': %s", name, err)
			}

			switch p.Status {
			case "running":
				return nil
			case "error":
				return fmt.Errorf("pipeline '%s' failed", name)
			default:
				log.Infof("pipeline '%s' is '%s'", name, p.Status)
			}
		}
	}
}

func getCurrentNamespace(ctx context.Context) (string, error) {
	c, _, namespace, err := k8Client.GetLocal("")
	if err != nil {
		log.Infof("couldn't get the current namespace: %s", err)
		return "", errors.UserError{
			E:    fmt.Errorf("couldn't get the current namespace"),
			Hint: "Run `okteto namespace`, or use the `--namespace` parameter",
		}
	}

	ns, err := namespaces.Get(namespace, c)
	if err != nil {
		return namespace, nil
	}

	if !namespaces.IsOktetoNamespace(ns) {
		return "", errors.UserError{
			E:    fmt.Errorf("your current namespace '%s' is not managed by okteto", namespace),
			Hint: "Run `okteto namespace`, or use the `--namespace` parameter",
		}
	}

	return namespace, nil
}
