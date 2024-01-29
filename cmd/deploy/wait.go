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

package deploy

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	ioCtrl "github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
)

type Waiter struct {
	K8sClientProvider okteto.K8sClientProviderWithLogger
	K8sLogger         *ioCtrl.K8sLogger
}

func NewDeployWaiter(k8sClientProvider okteto.K8sClientProviderWithLogger, k8slogger *ioCtrl.K8sLogger) Waiter {
	return Waiter{
		K8sClientProvider: k8sClientProvider,
		K8sLogger:         k8slogger,
	}
}

func (dw *Waiter) wait(ctx context.Context, opts *Options) error {
	oktetoLog.Spinner(fmt.Sprintf("Waiting for %s to be deployed...", opts.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)
	go func() {
		exit <- dw.waitForResourcesToBeRunning(ctx, opts)
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

func (dw *Waiter) waitForResourcesToBeRunning(ctx context.Context, opts *Options) error {
	ticker := time.NewTicker(5 * time.Second)
	to := time.NewTicker(opts.Timeout)
	c, _, err := dw.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, dw.K8sLogger)
	if err != nil {
		return err
	}

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' deploy didn't finish after %s", opts.Manifest.Name, opts.Timeout.String())
		case <-ticker.C:
			dList, err := pipeline.ListDeployments(ctx, opts.Manifest.Name, opts.Manifest.Namespace, c)
			if err != nil {
				return err
			}
			areAllRunning := true
			for i := range dList {
				d := &dList[i]
				if !deployments.IsRunning(ctx, opts.Manifest.Namespace, d.Name, c) {
					areAllRunning = false
				}
			}
			if !areAllRunning {
				continue
			}
			sfsList, err := pipeline.ListStatefulsets(ctx, opts.Manifest.Name, opts.Manifest.Namespace, c)
			if err != nil {
				return err
			}
			for i := range sfsList {
				ss := &sfsList[i]
				if !statefulsets.IsRunning(ctx, opts.Manifest.Namespace, ss.Name, c) {
					areAllRunning = false
				}
			}
			if !areAllRunning {
				continue
			}
			return nil
		}
	}
}
