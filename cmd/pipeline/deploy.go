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

package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// DeployOptions options for deploy pipeline command
type DeployOptions struct {
	Branch       string
	Repository   string
	Name         string
	Namespace    string
	Wait         bool
	SkipIfExists bool
	Timeout      time.Duration
	File         string
	Variables    []string

	//Deprecated fields
	Filename string
}

func deploy(ctx context.Context) *cobra.Command {
	opts := &DeployOptions{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy an okteto pipeline",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#deploy"),
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto pipeline deploy' is deprecated in favor of 'okteto deploy [--branch] [--repository]', and will be removed in a future version")
			return ExecuteDeployPipeline(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "p", "", "name of the pipeline (defaults to the git config name)")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().StringVarP(&opts.Repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().BoolVarP(&opts.Wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().BoolVarP(&opts.SkipIfExists, "skip-if-exists", "", false, "skip the pipeline deployment if the pipeline already exists in the namespace (defaults to false)")
	cmd.Flags().DurationVarP(&opts.Timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringArrayVarP(&opts.Variables, "var", "v", []string{}, "set a pipeline variable (can be set more than once)")
	cmd.Flags().StringVarP(&opts.File, "file", "f", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	cmd.Flags().StringVarP(&opts.Filename, "filename", "", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	cmd.Flags().MarkHidden("filename")
	return cmd
}

//ExecuteDeployPipeline executes deploy pipeline given a set of options
func ExecuteDeployPipeline(ctx context.Context, opts *DeployOptions) error {
	ctxResource := &model.ContextResource{}
	if err := ctxResource.UpdateNamespace(opts.Namespace); err != nil {
		return err
	}

	ctxOptions := &contextCMD.ContextOptions{
		Namespace: ctxResource.Namespace,
	}
	if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
		return err
	}

	if !okteto.IsOkteto() {
		return oktetoErrors.ErrContextIsNotOktetoCluster
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	if opts.Repository == "" {
		oktetoLog.Info("inferring git repository URL")

		opts.Repository, err = model.GetRepositoryURL(cwd)
		if err != nil {
			return err
		}
	}

	if opts.Name == "" {
		opts.Name = getPipelineName(opts.Repository)
	}

	currentRepo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		oktetoLog.Debug("cwd does not have .git folder")
	}

	if opts.Branch == "" && okteto.AreSameRepository(opts.Repository, currentRepo) {

		oktetoLog.Info("inferring git repository branch")
		b, err := utils.GetBranch(ctx, cwd)

		if err != nil {
			return err
		}

		opts.Branch = b
	}

	if opts.SkipIfExists {
		c, _, err := okteto.GetK8sClient()
		if err != nil {
			return fmt.Errorf("failed to load okteto context '%s': %v", okteto.Context().Name, err)
		}

		_, err = configmaps.Get(ctx, pipeline.TranslatePipelineName(opts.Name), okteto.Context().Namespace, c)
		if err == nil {
			oktetoLog.Success("Skipping '%s' because it's already deployed", opts.Name)
			return nil
		}

		if !oktetoErrors.IsNotFound(err) {
			return err
		}
	}

	if opts.Filename != "" {
		oktetoLog.Warning("the 'filename' flag is deprecated and will be removed in a future version. Please consider using 'file' flag")
		if opts.File == "" {
			opts.File = opts.Filename
		} else {
			oktetoLog.Warning("flags 'filename' and 'file' can not be used at the same time. 'file' flag will take precedence")
		}
	}

	resp, err := deployPipeline(ctx, opts)
	if err != nil {
		return err
	}
	oktetoLog.Information("Pipeline URL: %s", getPipelineURL(resp.GitDeploy))

	if !opts.Wait {
		oktetoLog.Success("Pipeline '%s' scheduled for deployment", opts.Name)
		return nil
	}

	if err := waitUntilRunning(ctx, opts.Name, resp.Action, opts.Timeout); err != nil {
		return err
	}
	oktetoLog.Success("Pipeline '%s' successfully deployed", opts.Name)
	return nil
}

func deployPipeline(ctx context.Context, opts *DeployOptions) (*types.GitDeployResponse, error) {
	spinner := utils.NewSpinner("Deploying your pipeline...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var err error
	var resp *types.GitDeployResponse
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}

	go func() {
		varList := []types.Variable{}
		for _, v := range opts.Variables {
			kv := strings.SplitN(v, "=", 2)
			if len(kv) != 2 {
				exit <- fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
				return
			}
			varList = append(varList, types.Variable{
				Name:  kv[0],
				Value: kv[1],
			})
		}
		namespace := okteto.Context().Namespace
		oktetoLog.Infof("deploy pipeline %s defined on file='%s' repository=%s branch=%s on namespace=%s", opts.Name, opts.File, opts.Repository, opts.Branch, namespace)

		resp, err = oktetoClient.DeployPipeline(ctx, opts.Name, opts.Repository, opts.Branch, opts.File, varList)
		exit <- err
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		return nil, oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return nil, err
		}
	}
	return resp, nil
}

func getPipelineName(repository string) string {
	return model.TranslateURLToName(repository)
}

func getPipelineURL(gitDeploy *types.GitDeploy) string {
	octx := okteto.Context()
	pipelineURL := fmt.Sprintf("%s/#/spaces/%s?resourceId=%s", octx.Name, octx.Namespace, gitDeploy.ID)
	return pipelineURL
}

func waitUntilRunning(ctx context.Context, name string, action *types.Action, timeout time.Duration) error {
	spinner := utils.NewSpinner(fmt.Sprintf("Waiting for %s to be deployed...", name))
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		err := waitToBeDeployed(ctx, name, action, timeout)
		if err != nil {
			exit <- err
			return
		}

		exit <- waitForResourcesToBeRunning(ctx, name, timeout)
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}

	return nil
}

func waitToBeDeployed(ctx context.Context, name string, action *types.Action, timeout time.Duration) error {
	if action == nil {
		return deprecatedWaitToBeDeployed(ctx, name, timeout)
	}
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	return oktetoClient.WaitForActionToFinish(ctx, name, action.Name, timeout)
}

//TODO: remove when all users are in Okteto Enterprise >= 0.10.0
func deprecatedWaitToBeDeployed(ctx context.Context, name string, timeout time.Duration) error {

	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	attempts := 0
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' deploy didn't finish after %s", name, timeout.String())
		case <-t.C:
			p, err := oktetoClient.GetPipelineByName(ctx, name)
			if err != nil {
				if oktetoErrors.IsNotFound(err) || oktetoErrors.IsNotExist(err) {
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
				oktetoLog.Infof("pipeline '%s' is '%s'", name, p.Status)
			}
		}
	}
}

func waitForResourcesToBeRunning(ctx context.Context, name string, timeout time.Duration) error {
	areAllRunning := false

	ticker := time.NewTicker(5 * time.Second)
	to := time.NewTicker(timeout)
	errorsMap := make(map[string]int)

	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' deploy didn't finish after %s", name, timeout.String())
		case <-ticker.C:
			resourceStatus, err := oktetoClient.GetResourcesStatusFromPipeline(ctx, name)
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
