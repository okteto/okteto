// Copyright 2022 The Okteto Authors
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
	"os/signal"
	"strings"
	"time"

	"github.com/docker/docker/pkg/namesgenerator"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// Deploy Deploy a preview environment
func Deploy(ctx context.Context) *cobra.Command {
	var branch string
	var filename string
	var file string
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
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{}); err != nil {
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

			okteto.Context().Namespace = name

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			if err := validatePreviewType(scope); err != nil {
				return err
			}

			varList := []types.Variable{}
			for _, v := range variables {
				kv := strings.SplitN(v, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
				}
				varList = append(varList, types.Variable{
					Name:  kv[0],
					Value: kv[1],
				})
			}

			if filename != "" {
				oktetoLog.Warning("the 'filename' flag is deprecated and will be removed in a future version. Please consider using 'file' flag'")
				if file == "" {
					file = filename
				} else {
					oktetoLog.Warning("flags 'filename' and 'file' can not be used at the same time. 'file' flag will take precedence")
				}
			}

			resp, err := executeDeployPreview(ctx, name, scope, repository, branch, sourceUrl, file, varList, wait, timeout)
			analytics.TrackPreviewDeploy(err == nil)
			if err != nil {
				return err
			}

			oktetoLog.Information("Preview URL: %s", getPreviewURL(name))
			if !wait {
				oktetoLog.Success("Preview environment '%s' scheduled for deployment", name)
				return nil
			}

			if err := waitUntilRunning(ctx, name, resp.Action, timeout); err != nil {
				return err
			}
			oktetoLog.Success("Preview environment '%s' successfully deployed", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().StringVarP(&repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "personal", "the scope of preview environment to create. Accepted values are ['personal', 'global']")
	cmd.Flags().StringVarP(&sourceUrl, "sourceUrl", "", "", "the URL of the original pull/merge request.")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringArrayVarP(&variables, "var", "v", []string{}, "set a preview environment variable (can be set more than once)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait until the preview environment deployment finishes (defaults to false)")
	cmd.Flags().StringVarP(&file, "file", "f", "", "relative path within the repository to the okteto manifest (default to okteto.yaml or .okteto/okteto.yaml)")

	cmd.Flags().StringVarP(&filename, "filename", "", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	cmd.Flags().MarkHidden("filename")
	return cmd
}

func validatePreviewType(previewType string) error {
	if !(previewType == "global" || previewType == "personal") {
		return fmt.Errorf("value '%s' is invalid for flag 'type'. Accepted values are ['global', 'personal']", previewType)
	}
	return nil
}

func getRepository(ctx context.Context, repository string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get the current working directory: %w", err)
	}

	if repository == "" {
		oktetoLog.Info("inferring git repository URL")

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
		oktetoLog.Info("inferring git repository branch")
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
		username := strings.ToLower(okteto.GetSanitizedUsername())
		name = fmt.Sprintf("%s-%s", name, username)
	}
	return name
}

func executeDeployPreview(ctx context.Context, name, scope, repository, branch, sourceUrl, filename string, variables []types.Variable, wait bool, timeout time.Duration) (*types.PreviewResponse, error) {
	oktetoLog.Spinner("Deploying your preview environment...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	resp, err := oktetoClient.DeployPreview(ctx, name, scope, repository, branch, sourceUrl, filename, variables)

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func waitUntilRunning(ctx context.Context, name string, a *types.Action, timeout time.Duration) error {
	oktetoLog.Spinner("Waiting for preview environment to be deployed...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		err := waitToBeDeployed(ctx, name, a, timeout)
		if err != nil {
			exit <- err
			return
		}
		oktetoLog.Spinner("Waiting for containers to be healthy...")
		exit <- waitForResourcesToBeRunning(ctx, name, timeout)
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		oktetoLog.StopSpinner()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}

	return nil
}
func waitToBeDeployed(ctx context.Context, name string, a *types.Action, timeout time.Duration) error {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	return oktetoClient.Pipeline().WaitForActionToFinish(ctx, name, a.Name, timeout)
}

func waitForResourcesToBeRunning(ctx context.Context, name string, timeout time.Duration) error {
	ticker := time.NewTicker(5 * time.Second)
	to := time.NewTicker(timeout)

	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}

	for {
		select {
		case <-to.C:
			return fmt.Errorf("preview environment '%s' didn't finish after %s", name, timeout.String())
		case <-ticker.C:
			resourceStatus, err := oktetoClient.GetResourcesStatusFromPreview(ctx, name, "")
			if err != nil {
				return err
			}
			allRunning, err := pipeline.CheckAllResourcesRunning(name, resourceStatus)
			if err != nil {
				return err
			}
			if allRunning {
				return nil
			}
		}
	}
}

func getExpandedName(name string) string {
	expandedName, err := model.ExpandEnv(name, true)
	if err != nil {
		return name
	}
	return expandedName
}

func getPreviewURL(name string) string {
	oktetoURL := okteto.Context().Name
	previewURL := fmt.Sprintf("%s/#/previews/%s", oktetoURL, name)
	return previewURL
}
