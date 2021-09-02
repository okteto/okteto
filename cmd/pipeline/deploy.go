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

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if repository == "" {
				log.Info("inferring git repository URL")

				repository, err = model.GetRepositoryURL(cwd)
				if err != nil {
					return err
				}
			}

			if name == "" {
				name = getPipelineName(repository)
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

			if skipIfExists {
				pipeline, err := okteto.GetPipelineByRepository(ctx, namespace, repository)
				if err == nil {
					log.Information("Pipeline URL: %s", getPipelineURL(namespace, pipeline))
					log.Success("Pipeline '%s' was already deployed", name)
					return nil
				}
				if !errors.IsNotFound(err) {
					return err
				}
			}

			pipeline, err := deployPipeline(ctx, name, namespace, repository, branch, filename, wait, variables)
			if err != nil {
				return err
			}
			log.Information("Pipeline URL: %s", getPipelineURL(namespace, pipeline))

			if !wait {
				log.Success("Pipeline '%s' scheduled for deployment", name)
				return nil
			}

			if waitUntilRunning(ctx, name, namespace, timeout); err != nil {
				return err
			}
			log.Success("Pipeline '%s' successfully deployed", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "p", "", "name of the pipeline (defaults to the git config name)")
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

func deployPipeline(ctx context.Context, name, namespace, repository, branch, filename string, wait bool, variables []string) (*okteto.PipelineRun, error) {
	spinner := utils.NewSpinner("Deploying your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var err error
	var pipeline *okteto.PipelineRun
	go func() {
		varList := []okteto.Variable{}
		for _, v := range variables {
			kv := strings.SplitN(v, "=", 2)
			if len(kv) != 2 {
				exit <- fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
			}
			varList = append(varList, okteto.Variable{
				Name:  kv[0],
				Value: kv[1],
			})
		}
		log.Infof("deploy pipeline %s defined on filename='%s' repository=%s branch=%s on namespace=%s", name, filename, repository, branch, namespace)
		pipeline, err = okteto.DeployPipeline(ctx, name, namespace, repository, branch, filename, varList)
		exit <- err
	}()

	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		os.Exit(130)
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return nil, err
		}
	}
	return pipeline, nil
}

func getPipelineName(repository string) string {
	return model.TranslateURLToName(repository)
}

func waitUntilRunning(ctx context.Context, name, namespace string, timeout time.Duration) error {
	spinner := utils.NewSpinner("Waiting for the pipeline to be deployed...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		err := waitToBeDeployed(ctx, name, namespace, timeout)
		if err != nil {
			exit <- err
		}

		exit <- waitForResourcesToBeRunning(ctx, name, namespace, timeout)
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

func waitToBeDeployed(ctx context.Context, name, namespace string, timeout time.Duration) error {

	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	attempts := 0

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

func waitForResourcesToBeRunning(ctx context.Context, name, namespace string, timeout time.Duration) error {
	areAllRunning := false

	ticker := time.NewTicker(5 * time.Second)
	to := time.NewTicker(timeout)
	errorsMap := make(map[string]int)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("pipeline '%s' didn't finish after %s", name, timeout.String())
		case <-ticker.C:
			resourceStatus, err := okteto.GetResourcesStatusFromPipeline(ctx, name, namespace)
			if err != nil {
				return err
			}
			areAllRunning = true
			for name, status := range resourceStatus {
				if status != "running" {
					areAllRunning = false
				}
				if status == "error" {
					errorsMap[name] = 1
				}
			}
			if len(errorsMap) > 0 {
				return fmt.Errorf("pipeline '%s' deployed with errors", name)
			}
			if areAllRunning {
				return nil
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

func getPipelineURL(namespace string, pipelineInfo *okteto.PipelineRun) string {
	oktetoURL := okteto.GetURL()
	pipelineURL := fmt.Sprintf("%s/#/spaces/%s?resourceId=%s", oktetoURL, namespace, pipelineInfo.ID)
	return pipelineURL
}
