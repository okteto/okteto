// Copyright 2023 The Okteto Authors
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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

var (
	ErrWaitResourcesTimeout = errors.New("preview environment didn't finish after on time")
)

type DeployOptions struct {
	branch             string
	deprecatedFilename string
	file               string
	name               string
	repository         string
	scope              string
	sourceUrl          string
	variables          []string
	labels             []string
	timeout            time.Duration
	wait               bool
}

// Deploy Deploy a preview environment
func Deploy(ctx context.Context) *cobra.Command {
	opts := &DeployOptions{}
	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Deploy a preview environment",
		Args:  utils.MaximumNArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if err := optionsSetup(cwd, opts, args); err != nil {
				return err
			}

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}
			oktetoLog.Information("Using %s @ %s as context", opts.name, okteto.RemoveSchema(okteto.GetContext().Name))

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			previewCmd, err := NewCommand()
			if err != nil {
				return err
			}
			return previewCmd.ExecuteDeployPreview(ctx, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().StringVarP(&opts.repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&opts.scope, "scope", "s", "personal", "the scope of preview environment to create. Accepted values are ['personal', 'global']")
	cmd.Flags().StringVarP(&opts.sourceUrl, "sourceUrl", "", "", "the URL of the original pull/merge request.")
	cmd.Flags().DurationVarP(&opts.timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringArrayVarP(&opts.variables, "var", "v", []string{}, "set a preview environment variable (can be set more than once)")
	cmd.Flags().BoolVarP(&opts.wait, "wait", "w", false, "wait until the preview environment deployment finishes (defaults to false)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "relative path within the repository to the okteto manifest (default to okteto.yaml or .okteto/okteto.yaml)")
	cmd.Flags().StringArrayVarP(&opts.labels, "label", "", []string{}, "set a preview environment label (can be set more than once)")

	cmd.Flags().StringVarP(&opts.deprecatedFilename, "filename", "", "", "relative path within the repository to the manifest file (default to okteto-pipeline.yaml or .okteto/okteto-pipeline.yaml)")
	if err := cmd.Flags().MarkHidden("filename"); err != nil {
		oktetoLog.Infof("failed to hide deprecated flag: %s", err)
	}
	return cmd
}

func (pw *Command) ExecuteDeployPreview(ctx context.Context, opts *DeployOptions) error {
	resp, err := pw.deployPreview(ctx, opts)
	analytics.TrackPreviewDeploy(err == nil, opts.scope)
	if err != nil {
		return err
	}

	oktetoLog.Information("Preview URL: %s", getPreviewURL(opts.name))
	if !opts.wait {
		oktetoLog.Success("Preview environment '%s' scheduled for deployment", opts.name)
		return nil
	}

	if err := pw.waitUntilRunning(ctx, opts.name, opts.name, resp.Action, opts.timeout); err != nil {
		return err
	}
	oktetoLog.Success("Preview environment '%s' successfully deployed", opts.name)
	return nil
}

func (pw *Command) deployPreview(ctx context.Context, opts *DeployOptions) (*types.PreviewResponse, error) {
	oktetoLog.Spinner("Deploying your preview environment...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	var varList []types.Variable
	for _, v := range opts.variables {
		variableFormatParts := 2
		kv := strings.SplitN(v, "=", variableFormatParts)
		if len(kv) != variableFormatParts {
			return nil, fmt.Errorf("invalid variable value '%s': must follow KEY=VALUE format", v)
		}
		varList = append(varList, types.Variable{
			Name:  kv[0],
			Value: kv[1],
		})
	}

	return pw.okClient.Previews().DeployPreview(ctx, opts.name, opts.scope, opts.repository, opts.branch, opts.sourceUrl, opts.file, varList, opts.labels)
}

func (pw *Command) waitUntilRunning(ctx context.Context, name, namespace string, a *types.Action, timeout time.Duration) error {
	waitCtx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()

	oktetoLog.Spinner("Waiting for preview environment to be deployed...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := pw.okClient.Stream().PipelineLogs(waitCtx, name, namespace, a.Name)
		if err != nil {
			oktetoLog.Warning("preview logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("preview logs cannot be streamed due to connectivity issues: %v", err)
		}
	}(&wg)

	wg.Add(1)
	go func() {
		err := pw.waitToBeDeployed(ctx, name, a, timeout)
		if err != nil {
			exit <- err
			return
		}
		oktetoLog.Spinner("Waiting for containers to be healthy...")
		exit <- pw.waitForResourcesToBeRunning(ctx, name, timeout)
	}()

	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(stop)
		close(exit)
	}(&wg)

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
func (pw *Command) waitToBeDeployed(ctx context.Context, name string, a *types.Action, timeout time.Duration) error {
	return pw.okClient.Pipeline().WaitForActionToFinish(ctx, name, name, a.Name, timeout)
}

func (pw *Command) waitForResourcesToBeRunning(ctx context.Context, name string, timeout time.Duration) error {
	ticker := time.NewTicker(5 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' %w - timeout %s", name, ErrWaitResourcesTimeout, timeout.String())
		case <-ticker.C:
			resourceStatus, err := pw.okClient.Previews().GetResourcesStatus(ctx, name, "")
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
