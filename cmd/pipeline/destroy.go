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
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// DestroyOptions options to destroy pipeline command
type DestroyOptions struct {
	Name           string
	Namespace      string
	Wait           bool
	DestroyVolumes bool
	Timeout        time.Duration
}

func destroy(ctx context.Context) *cobra.Command {
	opts := &DestroyOptions{}

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an okteto pipeline",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#destroy"),
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto pipeline destroy' is deprecated in favor of 'okteto destroy', and will be removed in a future version")
			return ExecuteDestroyPipeline(ctx, opts)

		},
	}

	cmd.Flags().StringVarP(&opts.Name, "name", "p", "", "name of the pipeline (defaults to the git config name)")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "namespace where the up command is executed (defaults to the current namespace)")
	cmd.Flags().BoolVarP(&opts.Wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().BoolVarP(&opts.DestroyVolumes, "volumes", "v", false, "destroy persistent volumes created by the pipeline (defaults to false)")
	cmd.Flags().DurationVarP(&opts.Timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	return cmd
}

//ExecuteDestroyPipeline executes destroy pipeline given a set of options
func ExecuteDestroyPipeline(ctx context.Context, opts *DestroyOptions) error {
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

	if opts.Name == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get the current working directory: %w", err)
		}
		repo, err := model.GetRepositoryURL(cwd)
		if err != nil {
			return err
		}

		opts.Name = getPipelineName(repo)
	}

	resp, err := destroyPipeline(ctx, opts.Name, opts.DestroyVolumes)
	if err != nil {
		return err
	}

	if !opts.Wait {
		oktetoLog.Success("Pipeline '%s' scheduled for destruction", opts.Name)
		return nil
	}

	if err := waitUntilDestroyed(ctx, opts.Name, resp.Action, opts.Timeout); err != nil {
		oktetoLog.Information("Pipeline URL: %s", getPipelineURL(resp.GitDeploy))
		return err
	}

	oktetoLog.Success("Pipeline '%s' successfully destroyed", opts.Name)

	return nil
}

func destroyPipeline(ctx context.Context, name string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	spinner := utils.NewSpinner("Destroying your pipeline...")
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
		resp, err = oktetoClient.DestroyPipeline(ctx, name, destroyVolumes)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				oktetoLog.Infof("pipeline '%s' not found", name)
				exit <- nil
				return
			}
			exit <- fmt.Errorf("failed to destroy pipeline '%s': %w", name, err)
			return
		}
		exit <- nil
	}()
	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return nil, err
		}
	}
	return resp, nil
}

func waitUntilDestroyed(ctx context.Context, name string, action *types.Action, timeout time.Duration) error {
	spinner := utils.NewSpinner("Waiting for the pipeline to be destroyed...")
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		exit <- waitToBeDestroyed(ctx, name, action, timeout)
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}

	return nil
}

func waitToBeDestroyed(ctx context.Context, name string, action *types.Action, timeout time.Duration) error {
	if action == nil {
		return deprecatedWaitToBeDestroyed(ctx, name, timeout)
	}
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	return oktetoClient.WaitForActionToFinish(ctx, name, action.Name, timeout)
}

//TODO: remove when all users are in Okteto Enterprise >= 0.10.0
func deprecatedWaitToBeDestroyed(ctx context.Context, name string, timeout time.Duration) error {

	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' destroy didn't finish after %s", name, timeout.String())
		case <-t.C:
			p, err := oktetoClient.GetPipelineByName(ctx, name)
			if err != nil {
				if oktetoErrors.IsNotFound(err) || oktetoErrors.IsNotExist(err) {
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
