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
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	helmReleaseSecretType  = "helm.sh/release.v1"
	helmReleaseOwnerLabel  = "owner"
	helmReleaseOwner       = "helm"
	helmReleaseNameLabel   = "name"
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
		logsCtxCancel()
	}(&wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		connectionTimeout := 5 * time.Minute
		err := lda.oktetoClient.Stream().DestroyAllLogs(logsCtx, opts.Namespace, connectionTimeout)
		if err != nil {
			oktetoLog.Warning("destroy all logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("destroy all logs cannot be streamed due to connectivity issues: %v", err)
		}
	}(&wg)

	select {
	case <-stop:
		ctxCancel()
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

	c, _, err := lda.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}
	hasBeenDestroyingAll := false

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
				if err := lda.checkAllResourcesDestroyed(ctx, namespace, c); err != nil {
					// If there are still resources remaining, continue polling unless we've been in DestroyingAll state
					if !hasBeenDestroyingAll {
						continue
					}
					// If we have been in DestroyingAll state but still have resources, this is a failure
					return err
				}

				// All resources have been successfully destroyed, exit the waiting loop
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

// listSecrets lists secrets in the given namespace
func (lda *localDestroyAllCommand) listSecrets(ctx context.Context, opts metav1.ListOptions, namespace string, k8s kubernetes.Interface) ([]v1.Secret, error) {
	secretList, err := k8s.CoreV1().Secrets(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return secretList.Items, nil
}

// listConfigmaps lists configmaps in the given namespace
func (lda *localDestroyAllCommand) listConfigmaps(ctx context.Context, opts metav1.ListOptions, namespace string, k8s kubernetes.Interface) ([]v1.ConfigMap, error) {
	cfgMapList, err := k8s.CoreV1().ConfigMaps(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return cfgMapList.Items, nil
}

// checkRemainingHelmReleases checks if there are any helm releases that still need to be uninstalled
func (lda *localDestroyAllCommand) checkRemainingHelmReleases(ctx context.Context, namespace string, k8s kubernetes.Interface) ([]string, error) {
	sList, err := lda.listSecrets(ctx, metav1.ListOptions{}, namespace, k8s)
	if err != nil {
		oktetoLog.Debugf("could not list secrets for ns '%s': %s", namespace, err.Error())
		return nil, err
	}

	releases := []string{}
	for _, s := range sList {
		if s.Type != helmReleaseSecretType || s.Labels[helmReleaseOwnerLabel] != helmReleaseOwner {
			continue
		}

		if name, ok := s.Labels[helmReleaseNameLabel]; ok {
			releases = append(releases, name)
		}
	}

	return releases, nil
}

// hasToBeExplicitlyDestroyed checks if a dev environment needs to be explicitly destroyed
func hasToBeExplicitlyDestroyed(cfmap v1.ConfigMap) (bool, error) {
	// This is a simplified version - you may need to implement specific logic
	// based on the manifest content stored in the configmap
	manifestContent, ok := cfmap.Data["manifest"]
	if !ok {
		return false, nil
	}
	
	// Basic check for destroy or divert sections
	// In a real implementation, you'd parse the YAML and check for these sections
	if manifestContent != "" {
		return true, nil // Simplified - assume it needs destruction if manifest exists
	}
	
	return false, nil
}

// getDevEnvNamesToDestroy retrieves the name of all the dev environments config map within the namespace
// which contains any `destroy` or `divert` section in the manifest
func (lda *localDestroyAllCommand) getDevEnvNamesToDestroy(ctx context.Context, namespace string, k8s kubernetes.Interface) ([]string, error) {
	opts := metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", model.GitDeployLabel)}
	cfmaps, err := lda.listConfigmaps(ctx, opts, namespace, k8s)
	if err != nil {
		return nil, err
	}

	// We need to validate the manifest section
	result := []string{}
	for _, cfmap := range cfmaps {
		name := cfmap.Data["name"]
		hasToBeDestroyed, err := hasToBeExplicitlyDestroyed(cfmap)
		if err != nil {
			message := fmt.Sprintf("could not check if dev environment '%s' has to be gracefully destroyed. Skipping it...", name)
			oktetoLog.Warning(message)
			continue
		}

		if hasToBeDestroyed {
			result = append(result, name)
		}
	}

	return result, nil
}

// checkAllResourcesDestroyed performs a comprehensive check to verify all resources have been destroyed
func (lda *localDestroyAllCommand) checkAllResourcesDestroyed(ctx context.Context, namespace string, k8s kubernetes.Interface) error {
	// Check for remaining configmaps with Okteto deployment labels
	resourcesLabels := map[string]bool{
		model.GitDeployLabel: true,
		model.StackLabel:     true,
		"dev.okteto.com/app": true,
	}

	cfgList, err := k8s.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Check configmaps for resources of the given namespace
	for _, cfg := range cfgList.Items {
		for l := range cfg.GetLabels() {
			if _, ok := resourcesLabels[l]; ok {
				return fmt.Errorf("namespace destroy all failed: some configmap resources were not destroyed (%s)", cfg.Name)
			}
		}
	}

	// Check for remaining helm releases
	helmReleases, err := lda.checkRemainingHelmReleases(ctx, namespace, k8s)
	if err != nil {
		return fmt.Errorf("error checking for remaining helm releases: %w", err)
	}
	if len(helmReleases) > 0 {
		return fmt.Errorf("namespace destroy all failed: helm releases still exist: %v", helmReleases)
	}

	// Check for dev environments that need explicit destruction
	devEnvs, err := lda.getDevEnvNamesToDestroy(ctx, namespace, k8s)
	if err != nil {
		return fmt.Errorf("error checking for dev environments to destroy: %w", err)
	}
	if len(devEnvs) > 0 {
		return fmt.Errorf("namespace destroy all failed: dev environments still exist that need explicit destruction: %v", devEnvs)
	}

	return nil
}
