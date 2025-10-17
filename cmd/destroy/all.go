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

package destroy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	destroyingAllTickerDuration = 30 * time.Second
)

type localDestroyAllCommand struct {
	oktetoClient      *okteto.Client
	k8sClientProvider okteto.K8sClientProvider
}

func newLocalDestroyerAll(
	k8sClientProvider okteto.K8sClientProvider,
	oktetoClient *okteto.Client,
) *localDestroyAllCommand {
	return &localDestroyAllCommand{
		k8sClientProvider: k8sClientProvider,
		oktetoClient:      oktetoClient,
	}
}

func (lda *localDestroyAllCommand) destroy(ctx context.Context, opts *Options) error {
	oktetoLog.Spinner(fmt.Sprintf("Deleting all in %s namespace", opts.Namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := lda.oktetoClient.Namespaces().DestroyAll(ctx, opts.Namespace, opts.DestroyVolumes); err != nil {
		return err
	}

	waitCtx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()

	logsCtx, logsCtxCancel := context.WithCancel(waitCtx)
	defer logsCtxCancel()

	stop := make(chan os.Signal, 1)
	defer close(stop)

	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)
	defer close(exit)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		exit <- lda.waitForNamespaceDestroyAllToComplete(waitCtx, opts.Namespace)
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		connectionTimeout := 5 * time.Minute
		err := lda.oktetoClient.Stream().DestroyAllLogs(logsCtx, opts.Namespace, connectionTimeout)

		// Check if error is not canceled because in the case of a timeout waiting the operation to complete,
		// we cancel the context to stop streaming logs, but we should not display the warning
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			oktetoLog.Warning("destroy all logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("destroy all logs cannot be streamed due to connectivity issues: %v", err)
		}
	}(&wg)

	select {
	case <-stop:
		ctxCancel()
		logsCtxCancel()
		oktetoLog.Infof("CTRL+C received, exit")
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		// wait until streaming logs have finished
		wg.Wait()
		return err
	}
}

// waitForNamespaceDestroyAllToComplete waits for the namespace destroy all operation to complete.
// When the namespace status is Active, it performs comprehensive checks to verify all resources
// have been properly destroyed, including helm releases and dev environments that need explicit destruction.
func (lda *localDestroyAllCommand) waitForNamespaceDestroyAllToComplete(ctx context.Context, namespace string) error {
	timeout := 5 * time.Minute
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)
	destroyingAllTicker := time.Now()

	c, _, err := lda.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}
	hasBeenDestroyingAll := false
	jobNotFoundAfterXSeconds := false

	for {
		select {
		case <-to.C:
			return fmt.Errorf("'%s' deploy didn't finish after %s", namespace, timeout.String())
		case <-ticker.C:
			ns, err := c.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				return err
			}

			status, ok := ns.Labels[constants.NamespaceStatusLabel]
			if !ok {
				// when status label is not present, continue polling the namespace until timeout
				oktetoLog.Debugf("namespace %q does not have label for status", namespace)
				continue
			}

			switch status {
			case "Active":
				// If we haven't been in DestroyingAll state for at least destroyingAllTickerDuration, wait before checking resources.
				if !hasBeenDestroyingAll && time.Since(destroyingAllTicker) < destroyingAllTickerDuration {
					jobNotFoundAfterXSeconds = true
				}

				if err := lda.checkAllResourcesDestroyed(ctx, namespace, c); err != nil {
					if jobNotFoundAfterXSeconds {
						continue
					}
					if hasBeenDestroyingAll {
						return err
					}
				}
				return nil
			case "DestroyingAll":
				// initial state would be active, check if this changes to assure namespace has been in destroying all status
				hasBeenDestroyingAll = true
			case "DestroyAllFailed":
				return errors.New("namespace destroy all failed")
			}
		}
	}
}

// checkAllResourcesDestroyed performs a comprehensive check to verify all resources have been destroyed
func (lda *localDestroyAllCommand) checkAllResourcesDestroyed(ctx context.Context, namespace string, k8s kubernetes.Interface) error {
	// when status is active again check if all resources have been correctly destroyed
	// list configmaps that belong okteto deployments
	resourcesLabels := map[string]bool{
		model.GitDeployLabel: true,
		model.StackLabel:     true,
		"dev.okteto.com/app": true,
	}

	cfgList, err := k8s.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	// no configmap for resources of the given namespace should exist
	for _, cfg := range cfgList.Items {
		for l := range cfg.GetLabels() {
			if _, ok := resourcesLabels[l]; ok {
				return fmt.Errorf("some resources were not destroyed")
			}
		}
	}

	// exit the waiting loop when status is active again
	return nil
}
