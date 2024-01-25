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

package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/devenvironment"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	modelUtils "github.com/okteto/okteto/pkg/model/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// destroyFlags represents the user input for a pipeline destroy command
type destroyFlags struct {
	name           string
	namespace      string
	wait           bool
	destroyVolumes bool
	timeout        time.Duration
}

// DestroyOptions options to destroy pipeline command
type DestroyOptions struct {
	Name           string
	Namespace      string
	Wait           bool
	DestroyVolumes bool
	Timeout        time.Duration
}

func destroy(ctx context.Context) *cobra.Command {
	flags := &destroyFlags{}

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an okteto pipeline",
		Args:  utils.NoArgsAccepted("https://www.okteto.com/docs/reference/cli/#destroy-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxResource := &model.ContextResource{}
			if err := ctxResource.UpdateNamespace(flags.namespace); err != nil {
				return err
			}

			ctxOptions := &contextCMD.Options{
				Namespace: ctxResource.Namespace,
				Show:      true,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			pipelineCmd, err := NewCommand()
			if err != nil {
				return err
			}
			opts := flags.toOptions()
			return pipelineCmd.ExecuteDestroyPipeline(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&flags.name, "name", "p", "", "name of the pipeline (defaults to the git config name)")
	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "namespace where the pipeline is destroyed (defaults to the current namespace)")
	cmd.Flags().BoolVarP(&flags.wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().BoolVarP(&flags.destroyVolumes, "volumes", "v", false, "destroy persistent volumes created by the pipeline (defaults to false)")
	cmd.Flags().DurationVarP(&flags.timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	return cmd
}

// ExecuteDestroyPipeline executes destroy pipeline given a set of options
func (pc *Command) ExecuteDestroyPipeline(ctx context.Context, opts *DestroyOptions) error {

	if err := opts.setDefaults(); err != nil {
		return fmt.Errorf("could not set default values for options: %w", err)
	}

	resp, err := pc.destroyPipeline(ctx, opts.Name, opts.Namespace, opts.DestroyVolumes)
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return err
	}

	if !opts.Wait {
		oktetoLog.Success("Repository '%s' scheduled for destruction", opts.Name)
		return nil
	}

	// If error is not found we don't have to wait
	if err == nil && resp != nil {
		if err := pc.waitUntilDestroyed(ctx, opts.Name, opts.Namespace, resp.Action, opts.Timeout); err != nil {
			return err
		}
	}

	oktetoLog.Success("Repository '%s' successfully destroyed", opts.Name)

	return nil
}

func (pc *Command) destroyPipeline(ctx context.Context, name, namespace string, destroyVolumes bool) (*types.GitDeployResponse, error) {
	oktetoLog.Spinner(fmt.Sprintf("Destroying repository '%s'...", name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var err error
	var resp *types.GitDeployResponse

	go func() {
		resp, err = pc.okClient.Pipeline().Destroy(ctx, name, namespace, destroyVolumes)
		if err != nil {
			exit <- fmt.Errorf("failed to destroy repository '%s': %w", name, err)
			return
		}
		exit <- nil
	}()
	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		oktetoLog.StopSpinner()
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return nil, err
		}
	}
	return resp, nil
}

func (pc *Command) waitUntilDestroyed(ctx context.Context, name, namespace string, action *types.Action, timeout time.Duration) error {
	waitCtx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()

	oktetoLog.Spinner(fmt.Sprintf("Waiting for the repository '%s' to be destroyed...", name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := pc.streamPipelineLogs(waitCtx, name, namespace, action.Name, timeout)
		if err != nil {
			oktetoLog.Warning("there was an error streaming pipeline logs: %v", err)
		}
	}(&wg)

	wg.Add(1)
	go func() {
		exit <- pc.waitToBeDestroyed(ctx, name, namespace, action, timeout)
	}()

	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(stop)
		close(exit)
	}(&wg)

	select {
	case <-stop:
		ctxCancel()
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

func (pc *Command) waitToBeDestroyed(ctx context.Context, name, namespace string, action *types.Action, timeout time.Duration) error {
	return pc.okClient.Pipeline().WaitForActionToFinish(ctx, name, namespace, action.Name, timeout)
}

// toOptions transform the flags
func (f destroyFlags) toOptions() *DestroyOptions {
	return &DestroyOptions{
		Name:           f.name,
		Namespace:      f.namespace,
		Wait:           f.wait,
		Timeout:        f.timeout,
		DestroyVolumes: f.destroyVolumes,
	}
}

func (o *DestroyOptions) setDefaults() error {
	if o.Name == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get the current working directory: %w", err)
		}
		repo, err := modelUtils.GetRepositoryURL(cwd)
		if err != nil {
			return err
		}

		c, _, err := okteto.NewK8sClientProvider().Provide(okteto.GetContext().Cfg)
		if err != nil {
			return err
		}
		inferer := devenvironment.NewNameInferer(c)
		// okteto pipeline destroy doesn't have a -f flag to specify the path, so we pass empty string
		o.Name = inferer.InferNameFromDevEnvsAndRepository(context.Background(), repo, okteto.GetContext().Namespace, "", "")
	}

	if o.Namespace == "" {
		o.Namespace = okteto.GetContext().Namespace
	}
	return nil
}
