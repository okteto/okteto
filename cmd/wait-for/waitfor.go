// Copyright 2025 The Okteto Authors
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

package waitfor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/endpoints"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// opts represents the wait-for command options
type opts struct {
	namespace      string
	k8sContext     string
	devEnvironment string
	timeout        time.Duration
}

// Cmd waits for services to be ready
func Cmd(ctx context.Context, k8sProvider okteto.K8sClientProvider, ioCtrl *io.Controller) *cobra.Command {
	var o opts
	cmd := &cobra.Command{
		Use:    "wait-for [kind/service/condition...]",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		Short:  "Waits for services to be ready",
		Long: `Waits for a specific resource to meet a given condition before continuing.

This command can be used with Deployments, StatefulSets, or Jobs by specifying the resource type, name, 
and the condition to wait for (e.g., "service_started", "service_healthy", or "service_completed").

Once the specified condition is met, the command exits successfully. If the condition is not met within 
the given timeframe, the command exits with an error.

Examples:

  1. okteto wait-for deployment/nginx/service_started
     - Waits for the "nginx" Deployment to reach the "service_started" condition.

  2. okteto wait-for statefulset/mysql/service_healthy
     - Waits for the "mysql" StatefulSet to become healthy.

  3. okteto wait-for job/wake/service_completed
     - Waits for the "wake" Job to finish successfully.

  4. okteto wait-for deployment/nginx/service_started statefulset/mysql/service_healthy job/wake/service_completed
     - Waits for all three resources to meet their respective conditions.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxOpts := &contextCMD.Options{
				Show:      true,
				Context:   o.k8sContext,
				Namespace: o.namespace,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			if !env.LoadBoolean(stack.OktetoComposeWaitForDependencies) {
				ioCtrl.Out().Infof("wait-for is not enabled in your stack file")
				return nil
			}

			deploymentList := map[string]model.DependsOnCondition{}
			stsList := map[string]model.DependsOnCondition{}
			jobsList := map[string]model.DependsOnCondition{}

			parser := newParser()
			for _, service := range args {
				result, err := parser.parse(service)
				if err != nil {
					return fmt.Errorf("invalid service '%s' format: %s", service, err)
				}
				switch result.serviceType {
				case "deployment":
					deploymentList[result.serviceName] = model.DependsOnCondition(result.condition)
				case "statefulset":
					stsList[result.serviceName] = model.DependsOnCondition(result.condition)
				case "job":
					jobsList[result.serviceName] = model.DependsOnCondition(result.condition)
				default:
					return fmt.Errorf("invalid resource type '%s'. The resource type must be 'deployment', 'statefulset', or 'job'", result.serviceType)
				}
				return nil
			}
			if len(deploymentList)+len(stsList)+len(jobsList) != len(args) {
				return fmt.Errorf("invalid service format. The service format must be 'kind/service/condition'")
			}
			var wg sync.WaitGroup
			wg.Add(len(args))

			c, _, err := k8sProvider.Provide(okteto.GetContext().Cfg)
			if err != nil {
				return fmt.Errorf("failed to get k8s client: %w", err)
			}

			for name, condition := range deploymentList {
				name := name
				condition := condition
				go waitForDeployment(ctx, c, name, condition, o.namespace, &wg, o.timeout, ioCtrl)
			}
			for name, condition := range stsList {
				name := name
				condition := condition
				go waitForStatefulSet(ctx, c, name, condition, o.namespace, &wg, o.timeout, ioCtrl)
			}
			for name, condition := range jobsList {
				name := name
				condition := condition
				go waitForJob(ctx, c, name, condition, o.namespace, &wg, o.timeout, ioCtrl)
			}
			wg.Wait()
			return nil
		},
	}

	cmd.Flags().StringVarP(&o.namespace, "namespace", "n", "", "namespace where the service is deployed")
	cmd.Flags().StringVarP(&o.devEnvironment, "dev-environment", "", "", "name of the development environment")
	cmd.Flags().StringVarP(&o.k8sContext, "context", "c", "", "overwrite the current Okteto Context")
	cmd.Flags().DurationVarP(&o.timeout, "timeout", "t", 5*time.Minute, "timeout to wait for the service to be ready")
	return cmd
}

func waitForDeployment(ctx context.Context, c kubernetes.Interface, resourceName string, condition model.DependsOnCondition, namespace string, wg *sync.WaitGroup, timeout time.Duration, ioCtrl *io.Controller) error {
	defer wg.Done()
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	isRunningCondition := condition == model.DependsOnServiceRunning
	for {
		select {
		case <-deadlineCtx.Done():
			return errors.New(fmt.Sprintf("Timeout waiting for deployment '%s'", resourceName))
		default:

			if deployments.IsRunning(ctx, namespace, resourceName, c) {
				if isRunningCondition {
					ioCtrl.Out().Success("Deployment '%s' is ready", resourceName)
					return nil
				}
				e, err := endpoints.GetByName(ctx, resourceName, namespace, c)
				if err != nil {
					continue
				}
				// if there are subsets, the service is healthy
				if len(e.Subsets) > 0 {
					ioCtrl.Out().Success("Deployment '%s' reached condition '%s'", resourceName, condition)
					return nil
				}
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func waitForStatefulSet(ctx context.Context, c kubernetes.Interface, name string, condition model.DependsOnCondition, namespace string, wg *sync.WaitGroup, timeout time.Duration, ioCtrl *io.Controller) error {
	defer wg.Done()
	to := time.Now().Add(timeout)
	ioCtrl.Logger().Infof("waiting for statefulset '%s' to reach condition '%s'", name, condition)

	switch condition {
	case model.DependsOnServiceRunning:
		for {
			if time.Now().After(to) {
				ioCtrl.Logger().Infof("statefulset '%s' did not reach condition '%s' after %s", name, condition, timeout)
				return fmt.Errorf("statefulset '%s' did not reach condition '%s' after %s", name, condition, timeout)
			}
			if statefulsets.IsRunning(ctx, namespace, name, c) {
				ioCtrl.Out().Success("statefulset '%s' reached condition '%s'", name, condition)
				return nil
			}

			time.Sleep(1 * time.Second)
		}
	case model.DependsOnServiceHealthy:
		for {
			if time.Now().After(to) {
				ioCtrl.Logger().Infof("statefulset '%s' did not reach condition '%s' after %s", name, condition, timeout)
				return fmt.Errorf("statefulset '%s' did not reach condition '%s' after %s", name, condition, timeout)
			}
			if statefulsets.IsRunning(ctx, namespace, name, c) {
				e, err := endpoints.GetByName(ctx, name, namespace, c)
				if err != nil {
					continue
				}
				// if there are subsets, the service is healthy
				if len(e.Subsets) > 0 {
					ioCtrl.Out().Success("statefulset '%s' reached condition '%s'", name, condition)
					return nil
				}
			}

			time.Sleep(5 * time.Second)
		}
	}
	return fmt.Errorf("unsupported condition '%s' for statefulset '%s'", condition, name)
}

func waitForJob(ctx context.Context, c kubernetes.Interface, name string, condition model.DependsOnCondition, namespace string, wg *sync.WaitGroup, timeout time.Duration, ioCtrl *io.Controller) error {
	defer wg.Done()
	to := time.Now().Add(timeout)
	ioCtrl.Logger().Infof("waiting for job '%s' to reach condition '%s'", name, condition)

	if condition != model.DependsOnServiceCompleted {
		return fmt.Errorf("unsupported condition '%s' for job '%s'", condition, name)
	}

	for {
		if time.Now().After(to) {
			ioCtrl.Logger().Infof("job '%s' did not reach condition '%s' after %s", name, condition, timeout)
			return fmt.Errorf("job '%s' did not reach condition '%s' after %s", name, condition, timeout)
		}
		job, err := c.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if job.Status.Succeeded == *job.Spec.Completions {
			ioCtrl.Out().Success("job '%s' reached condition '%s'", name, condition)
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}
