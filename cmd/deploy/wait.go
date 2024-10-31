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

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	ioCtrl "github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
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

func (dw *Waiter) wait(ctx context.Context, opts *Options, namespace string) error {
	oktetoLog.Spinner(fmt.Sprintf("Waiting for %s resources to be healthy...", opts.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	go func() {
		exit <- dw.waitForResourcesToBeRunning(ctx, opts, namespace)
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("%s resources where not healthy after %s", opts.Name, opts.Timeout)
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

func (dw *Waiter) waitForResourcesToBeRunning(ctx context.Context, opts *Options, namespace string) error {
	c, _, err := dw.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, dw.K8sLogger)
	if err != nil {
		return err
	}

	// first list the pods of the pipeline and check if they are running
	podList, err := pipeline.ListPods(ctx, namespace, opts.Manifest.Name, c)
	if err != nil {
		return err
	}
	pendingPodsMap := make(map[string]bool)
	for i := range podList.Items {
		p := &podList.Items[i]
		if p.Status.Phase != v1.PodRunning {
			pendingPodsMap[p.Name] = true
		}
	}

	// watch if there are pending pods to be running
	if len(pendingPodsMap) > 0 {
		w, err := pipeline.WatchPods(ctx, namespace, opts.Manifest.Name, podList.ResourceVersion, c)
		if err != nil {
			return err
		}
		defer w.Stop()

		for e := range w.ResultChan() {
			switch e.Type {
			case watch.Modified, watch.Added:
				pod, ok := e.Object.(*v1.Pod)
				if !ok {
					continue
				}
				if _, ok := pendingPodsMap[pod.Name]; ok && pod.Status.Phase == v1.PodRunning {
					delete(pendingPodsMap, pod.Name)
				}
				if len(pendingPodsMap) == 0 {
					return nil
				}
			// ignore this events
			case watch.Error:
				continue
			case watch.Deleted, watch.Bookmark:
				continue
			default:
				continue
			}
		}
	}

	return nil
}
