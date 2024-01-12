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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	dependencyEnvTemplate = "OKTETO_DEPENDENCY_%s_VARIABLE_%s"
)

var (
	errUnableToReuseParams = errors.New("development environment not found: unable to use --reuse-params option")
)

// Deployer deploys a pipeline
type Deployer struct {
	opts              *Options
	fs                afero.Fs
	pipelineCtrl      types.PipelineInterface
	streamCtrl        types.StreamInterface
	k8sClientProvider k8sClientProvider
	okCtxController   okCtxController
	ioCtrl            *io.IOController
}

type k8sClientProvider interface {
	Provide(clientApiConfig *api.Config) (kubernetes.Interface, *rest.Config, error)
}

type okCtxController interface {
	GetNamespace() string
	GetK8sConfig() *api.Config
}

// NewPipelineDeployer returns a new pipeline deployer
func NewPipelineDeployer(o ...OptFunc) (*Deployer, error) {
	opts := &Options{}
	for _, of := range o {
		of(opts)
	}
	if err := opts.setDefaults(); err != nil {
		return nil, fmt.Errorf("could not set default values for options: %w", err)
	}
	return &Deployer{
		opts:              opts,
		fs:                opts.fs,
		k8sClientProvider: opts.k8sClientProvider,
		okCtxController:   opts.okCtxController,
		ioCtrl:            opts.ioCtrl,
		pipelineCtrl:      opts.okClient.Pipeline(),
		streamCtrl:        opts.okClient.Stream(),
	}, nil
}

// Deploy deploys a pipeline
func (d *Deployer) Deploy(ctx context.Context) error {
	c, _, err := d.k8sClientProvider.Provide(d.okCtxController.GetK8sConfig())
	if err != nil {
		return fmt.Errorf("failed to load okteto context '%s': %w", okteto.Context().Name, err)
	}

	exists := false
	cfgName := pipeline.TranslatePipelineName(d.opts.Name)
	cfg, err := configmaps.Get(ctx, cfgName, d.opts.Namespace, c)
	if err != nil {
		if d.opts.ReuseParams && oktetoErrors.IsNotFound(err) {
			return errUnableToReuseParams
		}
		if d.opts.SkipIfExists && !oktetoErrors.IsNotFound(err) {
			return fmt.Errorf("failed to get pipeline '%s': %w", cfgName, err)
		}
	}
	exists = cfg != nil && cfg.Data != nil

	if d.opts.ReuseParams && exists {
		d.opts.SetFromCmap(cfg)
	}

	if d.opts.SkipIfExists && exists {
		if cfg.Data["status"] == pipeline.DeployedStatus {
			d.ioCtrl.Out().Success("Skipping repository '%s' because it's already deployed", d.opts.Name)
			return nil
		}

		if !d.opts.Wait && cfg.Data["status"] == pipeline.ProgressingStatus {
			d.ioCtrl.Out().Success("Repository '%s' already scheduled for deployment", d.opts.Name)
			return nil
		}

		canStreamPrevLogs := cfg.Data["actionLock"] != "" && cfg.Data["actionName"] != "cli"

		if d.opts.Wait && canStreamPrevLogs {
			sp := d.ioCtrl.Out().Spinner(fmt.Sprintf("Repository '%s' is already being deployed, waiting for it to finish...", d.opts.Name))
			sp.Start()
			defer sp.Stop()

			existingAction := &types.Action{
				ID:   cfg.Data["actionLock"],
				Name: cfg.Data["actionName"],
			}
			if err := d.waitUntilRunning(ctx, d.opts.Name, d.opts.Namespace, existingAction, d.opts.Timeout); err != nil {
				return fmt.Errorf("wait for pipeline '%s' to finish failed: %w", d.opts.Name, err)
			}
			d.ioCtrl.Out().Success("Repository '%s' successfully deployed", d.opts.Name)
			return nil
		}

		if d.opts.Wait && !canStreamPrevLogs && cfg.Data["status"] == pipeline.ProgressingStatus {
			sp := d.ioCtrl.Out().Spinner(fmt.Sprintf("Repository '%s' is already being deployed, waiting for it to finish...", d.opts.Name))
			sp.Start()
			defer sp.Stop()

			ticker := time.NewTicker(1 * time.Second)
			err := configmaps.WaitForStatus(ctx, cfgName, d.opts.Namespace, pipeline.DeployedStatus, ticker, d.opts.Timeout, c)
			if err != nil {
				if errors.Is(err, oktetoErrors.ErrTimeout) {
					return fmt.Errorf("timed out waiting for repository '%s' to be deployed", d.opts.Name)
				}
				return fmt.Errorf("failed to wait for repository '%s' to be deployed: %w", d.opts.Name, err)
			}

			d.ioCtrl.Out().Success("Repository '%s' successfully deployed", d.opts.Name)
			return nil
		}
	}

	resp, err := d.deployPipeline(ctx, d.opts)
	if err != nil {
		return fmt.Errorf("failed to deploy pipeline '%s': %w", d.opts.Name, err)
	}

	if !d.opts.Wait {
		d.ioCtrl.Out().Success("Repository '%s' scheduled for deployment", d.opts.Name)
		return nil
	}

	sp := d.ioCtrl.Out().Spinner(fmt.Sprintf("Waiting for repository '%s' to be deployed...", d.opts.Name))
	sp.Start()
	defer sp.Stop()

	if err := d.waitUntilRunning(ctx, d.opts.Name, d.opts.Namespace, resp.Action, d.opts.Timeout); err != nil {
		return fmt.Errorf("wait for pipeline '%s' to finish failed: %w", d.opts.Name, err)
	}

	cmap, err := configmaps.Get(ctx, cfgName, d.opts.Namespace, c)
	if err != nil {
		return err
	}

	if err := setEnvsFromDependency(cmap, os.Setenv); err != nil {
		return fmt.Errorf("could not set environment variable generated by dependency '%s': %w", d.opts.Name, err)
	}

	d.ioCtrl.Out().Success("Repository '%s' successfully deployed", d.opts.Name)
	return nil
}

type envSetter func(name, value string) error

// setEnvsFromDependency sets the environment variables found at configmap.Data[dependencyEnvs]
func setEnvsFromDependency(cmap *v1.ConfigMap, envSetter envSetter) error {
	if cmap == nil || cmap.Data == nil {
		return nil
	}

	dependencyEnvsEncoded, ok := cmap.Data[constants.OktetoDependencyEnvsKey]
	if !ok {
		return nil
	}

	name := cmap.Name

	decodedEnvs, err := base64.StdEncoding.DecodeString(dependencyEnvsEncoded)
	if err != nil {
		return err
	}
	envsToSet := make(map[string]string)
	if err = json.Unmarshal(decodedEnvs, &envsToSet); err != nil {
		return err
	}
	for envKey, envValue := range envsToSet {
		envName := fmt.Sprintf(dependencyEnvTemplate, strings.ToUpper(name), envKey)
		if err := envSetter(envName, envValue); err != nil {
			return err
		}
	}

	return nil
}

func (d *Deployer) deployPipeline(ctx context.Context, opts *Options) (*types.GitDeployResponse, error) {
	sp := d.ioCtrl.Out().Spinner(fmt.Sprintf("Deploying repository '%s'...", opts.Name))
	sp.Start()
	defer sp.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var resp *types.GitDeployResponse

	go func() {

		pipelineOpts, err := opts.toPipelineDeployClientOptions()
		if err != nil {
			exit <- err
			return
		}
		d.ioCtrl.Logger().Infof("deploy pipeline %s defined on file='%s' repository=%s branch=%s on namespace=%s", opts.Name, opts.File, opts.Repository, opts.Branch, opts.Namespace)

		resp, err = d.pipelineCtrl.Deploy(ctx, pipelineOpts)
		exit <- err
	}()

	select {
	case <-stop:
		d.ioCtrl.Logger().Infof("CTRL+C received, starting shutdown sequence")
		return nil, oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			d.ioCtrl.Logger().Infof("exit signal received due to error: %s", err)
			return nil, err
		}
	}
	return resp, nil
}

func (d *Deployer) streamPipelineLogs(ctx context.Context, name, namespace, actionName string, timeout time.Duration) error {
	// wait to Action be progressing
	if err := d.pipelineCtrl.WaitForActionProgressing(ctx, name, namespace, actionName, timeout); err != nil {
		return err
	}

	return d.streamCtrl.PipelineLogs(ctx, name, namespace, actionName)
}

func (d *Deployer) waitUntilRunning(ctx context.Context, name, namespace string, action *types.Action, timeout time.Duration) error {
	waitCtx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := d.streamPipelineLogs(waitCtx, name, namespace, action.Name, timeout)
		if err != nil {
			d.ioCtrl.Out().Warning("pipeline logs cannot be streamed due to connectivity issues")
			d.ioCtrl.Logger().Infof("pipeline logs cannot be streamed due to connectivity issues: %v", err)
		}
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := d.pipelineCtrl.WaitForActionToFinish(ctx, name, namespace, action.Name, timeout)
		if err != nil {
			exit <- err
			return
		}

		exit <- d.waitForResourcesToBeRunning(waitCtx, name, namespace, timeout)
	}(&wg)

	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(stop)
		close(exit)
	}(&wg)

	select {
	case <-stop:
		ctxCancel()
		d.ioCtrl.Logger().Infof("CTRL+C received, starting shutdown sequence")
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			d.ioCtrl.Logger().Infof("exit signal received due to error: %s", err)
			return err
		}
	}

	return nil
}

type ErrTimeout struct {
	name    string
	timeout time.Duration
}

func (e *ErrTimeout) Error() string {
	return fmt.Sprintf("'%s' deploy didn't finish after %s", e.name, e.timeout.String())
}

func (d *Deployer) waitForResourcesToBeRunning(ctx context.Context, name, namespace string, timeout time.Duration) error {
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return &ErrTimeout{name, timeout}
		case <-ticker.C:
			resourceStatus, err := d.pipelineCtrl.GetResourcesStatus(ctx, name, namespace)
			if err != nil {
				return err
			}
			allRunning, err := d.checkAllResourcesRunning(name, resourceStatus)
			if err != nil {
				return err
			}
			if allRunning {
				return nil
			}
		}
	}
}

func (d *Deployer) checkAllResourcesRunning(name string, resourceStatus map[string]string) (bool, error) {
	allRunning := true
	for resourceID, status := range resourceStatus {
		d.ioCtrl.Logger().Infof("Resource %s is %s", resourceID, status)
		if status == okteto.ErrorStatus {
			return false, fmt.Errorf("repository '%s' deployed with errors", name)
		}
		if okteto.TransitionStatus[status] {
			allRunning = false
		}
	}
	return allRunning, nil
}
