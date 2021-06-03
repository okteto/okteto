// Copyright 2020 The Okteto Authors
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

package stack

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/jobs"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func Wait(ctx context.Context, stackName, svcName, namespace string) error {

	if stackName == "" {
		return fmt.Errorf("Invalid command: stack name must is required.")
	}
	if svcName == "" {
		return fmt.Errorf("Invalid command: service name is required.")
	}

	c, _, err := client.GetLocal()
	if err != nil {
		return err
	}

	configMap, err := configmaps.Get(ctx, model.GetStackConfigMapName(stackName), namespace, c)
	if err != nil {
		return fmt.Errorf("Invalid command: stack '%s' does not exist on namespace '%s'.", stackName, namespace)
	}

	s, err := translateConfigMapToStack(configMap)

	if svc, ok := s.Services[svcName]; ok {
		addHiddenExposedPortsToDependentSvcs(ctx, s, svc)
		err = waitForSvcs(ctx, s, svc, c)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Invalid command: stack '%s' does not exist.", stackName)
	}

	return nil
}

func translateConfigMapToStack(configMap *apiv1.ConfigMap) (*model.Stack, error) {
	isCompose, err := strconv.ParseBool(configMap.Data[ComposeField])
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(configMap.Data[YamlField])
	if err != nil {
		return nil, err
	}

	stack, err := model.ReadStack(decoded, isCompose)
	if err != nil {
		return nil, err
	}
	stack.Name = configMap.Data[NameField]
	stack.Namespace = configMap.Namespace
	return stack, nil
}

func addHiddenExposedPortsToDependentSvcs(ctx context.Context, s *model.Stack, svc *model.Service) {
	for dependentSvcName := range svc.DependsOn {
		addHiddenExposedPortsToSvc(ctx, s.Services[dependentSvcName], s.Namespace)
	}
}

func waitForSvcs(ctx context.Context, stack *model.Stack, svc *model.Service, c kubernetes.Interface) error {
	var err error
	for dependentSvc, condition := range svc.DependsOn {
		fmt.Printf("Waiting for service '%s'\n", dependentSvc)
		switch condition.Condition {
		case model.DependsOnServiceRunning:
			err = waitForSvcRunning(ctx, stack, dependentSvc, c)
			if err != nil {
				return err
			}
		case model.DependsOnServiceHealthy:
			err = waitForSvcHealthy(stack, dependentSvc)
			if err != nil {
				return err
			}
		case model.DependsOnServiceCompleted:
			err = waitForSvcCompleted(ctx, stack, dependentSvc, c)
			if err != nil {
				return err
			}
		}
		fmt.Printf("Service '%s' is ready\n", dependentSvc)
	}
	return nil
}

func waitForSvcRunning(ctx context.Context, stack *model.Stack, svcName string, c kubernetes.Interface) error {
	svcToWaitFor := stack.Services[svcName]

	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.Now().Add(300 * time.Second)
	isDeployment := isDeployment(svcToWaitFor)
	isStatefulset := isStatefulset(svcToWaitFor)

	for time.Now().Before(timeout) {
		<-ticker.C
		switch {
		case isDeployment:
			if deployments.IsRunning(ctx, stack.Namespace, svcName, c) {
				return nil
			}
		case isStatefulset:
			if statefulsets.IsRunning(ctx, stack.Namespace, svcName, c) {
				return nil
			}
		default:
			if jobs.IsRunning(ctx, stack.Namespace, svcName, c) {
				return nil
			}
		}
	}
	return fmt.Errorf("Service '%s' is taking too long to be healthy. Please check logs and try again later.", svcName)
}

func isDeployment(svc *model.Service) bool {
	return len(svc.Volumes) == 0 && svc.RestartPolicy == apiv1.RestartPolicyAlways
}
func isStatefulset(svc *model.Service) bool {
	return len(svc.Volumes) != 0 && svc.RestartPolicy == apiv1.RestartPolicyAlways
}

func waitForSvcHealthy(stack *model.Stack, svcName string) error {
	svcToWaitFor := stack.Services[svcName]

	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.Now().Add(300 * time.Second)

	for time.Now().Before(timeout) {
		<-ticker.C
		for _, p := range svcToWaitFor.Ports {
			url := fmt.Sprintf("http://%s:%d/", svcName, p.Port)
			resp, err := http.Get(url)
			if err != nil {
				continue
			}
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				return nil
			}
		}
	}
	return fmt.Errorf("Service '%s' is taking too long to be healthy. Please check logs and try again later.", svcName)
}

func waitForSvcCompleted(ctx context.Context, stack *model.Stack, svcName string, c kubernetes.Interface) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.Now().Add(300 * time.Second)

	for time.Now().Before(timeout) {
		<-ticker.C
		if jobs.IsSuccedded(ctx, stack.Namespace, svcName, c) {
			return nil
		}
	}
	return fmt.Errorf("Service '%s' is taking too long to be healthy. Please check logs and try again later.", svcName)
}
