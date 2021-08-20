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

package preview

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Deploy Deploy a preview environment
func Deploy(ctx context.Context) *cobra.Command {
	var branch string
	var filename string
	var name string
	var repository string
	var scope string
	var sourceUrl string
	var timeout time.Duration
	var variables []string
	var wait bool

	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Deploy a preview environment",
		Args:  utils.MaximumNArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			if !okteto.IsAuthenticated() {
				return okErrors.ErrNotLogged
			}

			if err := validatePreviewType(scope); err != nil {
				return err
			}

			var err error
			repository, err = getRepository(ctx, repository)
			if err != nil {
				return err
			}
			branch, err = getBranch(ctx, branch)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				name = getRandomName(ctx, scope)
			} else {
				name = getExpandedName(args[0])
			}

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

			name, err = executeDeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, varList, wait, timeout)
			analytics.TrackPreviewDeploy(err == nil)
			if err != nil {
				return err
			}

			if !wait {
				log.Success("Preview environment '%s' scheduled for deployment", name)
				return nil
			}

			if err := waitUntilRunning(ctx, name, name, timeout); err != nil {
				return fmt.Errorf("preview deployed with resource errors")
			}
			log.Success("Preview environment '%s' successfully deployed", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "personal", "the scope of preview environment to create. Accepted values are ['personal', 'global']")
	cmd.Flags().StringVarP(&sourceUrl, "sourceUrl", "", "", "the URL of the original pull/merge request.")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringArrayVarP(&variables, "var", "v", []string{}, "set a pipeline variable (can be set more than once)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the preview environment deployment finishes (defaults to false)")

	return cmd
}

func validatePreviewType(previewType string) error {
	if !(previewType == "global" || previewType == "personal") {
		return fmt.Errorf("Value '%s' is invalid for flag 'type'. Accepted values are ['global', 'personal']", previewType)
	}
	return nil
}

func getRepository(ctx context.Context, repository string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get the current working directory: %w", err)
	}

	if repository == "" {
		log.Info("inferring git repository URL")

		r, err := model.GetRepositoryURL(cwd)

		if err != nil {
			return "", err
		}

		repository = r
	}
	return repository, nil
}

func getBranch(ctx context.Context, branch string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get the current working directory: %w", err)
	}
	if branch == "" {
		log.Info("inferring git repository branch")
		b, err := utils.GetBranch(ctx, cwd)

		if err != nil {
			return "", err
		}
		branch = b
	}
	return branch, nil
}

func getRandomName(ctx context.Context, scope string) string {
	name := strings.ReplaceAll(namesgenerator.GetRandomName(-1), "_", "-")
	if scope == "personal" {
		username := strings.ToLower(okteto.GetUsername())
		name = fmt.Sprintf("%s-%s", name, username)
	}
	return name
}

func executeDeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []okteto.Variable, wait bool, timeout time.Duration) (string, error) {
	spinner := utils.NewSpinner("Deploying your preview environment...")
	spinner.Start()
	defer spinner.Stop()

	oktetoNS, err := okteto.DeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, variables)
	if err != nil {
		return "", err
	}
	return oktetoNS, nil
}

func waitUntilRunning(ctx context.Context, name, namespace string, timeout time.Duration) error {
	spinner := utils.NewSpinner("Waiting for the preview environment to finish...")
	spinner.Start()
	defer spinner.Stop()
	err := waitToBeDeployed(ctx, name, namespace, timeout)
	if err != nil {
		return err
	}

	err = waitForResourcesToBeRunning(ctx, name, namespace, timeout)
	if err != nil {
		return err
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
			return fmt.Errorf("preview environment '%s' didn't finish after %s", name, timeout.String())
		case <-t.C:
			p, err := okteto.GetPreviewEnvByName(ctx, name, namespace)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsNotExist(err) {
					return nil
				}

				return fmt.Errorf("failed to get preview environment '%s': %s", name, err)
			}

			switch p.Status {
			case "deployed", "running":
				return nil
			case "error":
				attempts++
				if attempts > 30 {
					return fmt.Errorf("preview environment '%s' failed", name)
				}
			default:
				log.Infof("preview environment '%s' is '%s'", name, p.Status)
			}
		}
	}
}

func waitForResourcesToBeRunning(ctx context.Context, name, namespace string, timeout time.Duration) error {
	areAllRunning := false

	ticker := time.NewTicker(5 * time.Second)
	to := time.NewTicker(timeout)
	errors := make(map[string]int)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("preview environment '%s' didn't finish after %s", name, timeout.String())
		case <-ticker.C:
			resourceStatus, err := okteto.GetResourcesStatusFromPreview(ctx, namespace)
			if err != nil {
				return err
			}
			areAllRunning = true
			for name, status := range resourceStatus {
				if status != "running" {
					areAllRunning = false
				}
				if status == "error" {
					errors[name] = 1
				}
			}
			if len(errors) > 0 {
				return fmt.Errorf("preview environment '%s' deployed with resource errors", name)
			}
			if areAllRunning {
				return nil
			}
		}
	}
}

func getExpandedName(name string) string {
	expandedName, err := model.ExpandEnv(name)
	if err != nil {
		return name
	}
	return expandedName
}
