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
	"path/filepath"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

func deploy(ctx context.Context) *cobra.Command {
	var branch string
	var repository string
	var name string
	var namespace string
	var wait bool
	var skipIfExists bool
	var timeout time.Duration
	var variables []string
	var filename string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys an okteto pipeline",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#deploy"),
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

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if repository == "" {
				log.Info("inferring git repository URL")

				r, err := model.GetRepositoryURL(cwd)

				if err != nil {
					return err
				}

				repository = r

			}

			if branch == "" {
				log.Info("inferring git repository branch")
				b, err := utils.GetBranch(ctx, cwd)

				if err != nil {
					return err
				}

				branch = b
			}

			if namespace == "" {
				namespace = getCurrentNamespace(ctx)
			}

			currentContext := client.GetSessionContext("")
			if okteto.GetClusterContext() != currentContext {
				log.Information("Pipeline context: %s", okteto.GetURL())
			}

			if skipIfExists {
				_, err := okteto.GetPipelineByRepository(ctx, namespace, repository)
				if err == nil {
					log.Success("Pipeline '%s' was already deployed", name)
					return nil
				}
				if !errors.IsNotFound(err) {
					return err
				}
			}

			if err := deployPipeline(ctx, name, namespace, repository, branch, filename, wait, timeout, variables); err != nil {
				return err
			}

			if wait {
				log.Success("Pipeline '%s' successfully deployed", name)
			} else {
				log.Success("Pipeline '%s' scheduled for deployment", name)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "p", "", "name of the pipeline (defaults to the folder name)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().BoolVarP(&skipIfExists, "skip-if-exists", "", false, "skip the pipeline deployment if the pipeline already exists in the namespace (defaults to false)")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringArrayVarP(&variables, "var", "v", []string{}, "set a pipeline variable (can be set more than once)")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	return cmd
}

func deployPipeline(ctx context.Context, name, namespace, repository, branch, filename string, wait bool, timeout time.Duration, variables []string) error {
	spinner := utils.NewSpinner("Deploying your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	varList := []okteto.Variable{}
	for _, v := range variables {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		varList = append(varList, okteto.Variable{
			Name:  kv[0],
			Value: kv[1],
		})
	}
	log.Infof("deploy pipeline %s defined on filename='%s' repository=%s branch=%s on namespace=%s", name, filename, repository, branch, namespace)
	_, err := okteto.DeployPipeline(ctx, name, namespace, repository, branch, filename, varList)
	if err != nil {
		return fmt.Errorf("failed to deploy pipeline: %w", err)
	}

	if !wait {
		return nil
	}

	spinner.Update("Waiting for the pipeline to finish...")
	return waitUntilRunning(ctx, name, namespace, timeout)
}

func getPipelineName() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Base(workDir), nil
}

func waitUntilRunning(ctx context.Context, name, namespace string, timeout time.Duration) error {
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	attempts := 0

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
			case "deployed", "running":
				return nil
			case "error":
				attempts++
				if attempts > 30 {
					return fmt.Errorf("pipeline '%s' failed", name)
				}
			default:
				log.Infof("pipeline '%s' is '%s'", name, p.Status)
			}
		}
	}
}

func getCurrentNamespace(ctx context.Context) string {
	currentContext := client.GetSessionContext("")
	if okteto.GetClusterContext() == currentContext {
		return client.GetContextNamespace("")
	}
	return os.Getenv("OKTETO_NAMESPACE")
}
