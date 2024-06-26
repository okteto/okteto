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

package stack

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/jobs"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/kubernetes"
)

func destroyServicesNotInStack(ctx context.Context, s *model.Stack, c kubernetes.Interface) error {
	if err := destroyDeployments(ctx, s, c); err != nil {
		return err
	}

	if err := destroyStatefulsets(ctx, s, c); err != nil {
		return err
	}

	if err := destroyJobs(ctx, s, c); err != nil {
		return err
	}

	err := destroyIngresses(ctx, s, c)
	if err != nil {
		return err
	}

	return nil
}

func destroyDeployments(ctx context.Context, s *model.Stack, c kubernetes.Interface) error {
	dList, err := deployments.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range dList {
		if _, ok := s.Services[dList[i].Name]; ok && s.Services[dList[i].Name].IsDeployment() {
			continue
		}
		if err := deployments.Destroy(ctx, dList[i].Name, dList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying deployment of service '%s': %w", dList[i].Name, err)
		}
		if err := services.Destroy(ctx, dList[i].Name, dList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %w", dList[i].Name, err)
		}
		if _, ok := s.Services[dList[i].Name]; ok {
			oktetoLog.Success("Destroyed previous service '%s'", dList[i].Name)
		} else {
			oktetoLog.Success("Service '%s' destroyed", dList[i].Name)
		}
	}
	return nil
}

func destroyStatefulsets(ctx context.Context, s *model.Stack, c kubernetes.Interface) error {
	sfsList, err := statefulsets.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range sfsList {
		if _, ok := s.Services[sfsList[i].Name]; ok && s.Services[sfsList[i].Name].IsStatefulset() {
			continue
		}
		if err := statefulsets.Destroy(ctx, sfsList[i].Name, sfsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying statefulset of service '%s': %w", sfsList[i].Name, err)
		}
		if err := services.Destroy(ctx, sfsList[i].Name, sfsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %w", sfsList[i].Name, err)
		}
		if _, ok := s.Services[sfsList[i].Name]; ok {
			oktetoLog.Success("Destroyed previous service '%s'", sfsList[i].Name)
		} else {
			oktetoLog.Success("Service '%s' destroyed", sfsList[i].Name)
		}
	}
	return nil
}
func destroyJobs(ctx context.Context, s *model.Stack, c kubernetes.Interface) error {
	jobsList, err := jobs.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range jobsList {
		if _, ok := s.Services[jobsList[i].Name]; ok && s.Services[jobsList[i].Name].IsJob() {
			continue
		}
		if err := jobs.Destroy(ctx, jobsList[i].Name, jobsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying job of service '%s': %w", jobsList[i].Name, err)
		}
		if err := services.Destroy(ctx, jobsList[i].Name, jobsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %w", jobsList[i].Name, err)
		}
		oktetoLog.StopSpinner()
		if _, ok := s.Services[jobsList[i].Name]; ok {
			oktetoLog.Success("Destroyed previous service '%s'", jobsList[i].Name)
		} else {
			oktetoLog.Success("Service '%s' destroyed", jobsList[i].Name)
		}
		oktetoLog.StartSpinner()
	}
	return nil
}

func destroyIngresses(ctx context.Context, s *model.Stack, c kubernetes.Interface) error {
	iClient, err := ingresses.GetClient(c)
	if err != nil {
		return fmt.Errorf("error getting ingress client: %w", err)
	}

	iList, err := iClient.List(ctx, s.Namespace, s.GetLabelSelector())
	if err != nil {
		return err
	}
	publicSvcsMap := map[string]bool{}
	for svcName, svcInfo := range s.Services {
		if len(svcInfo.Ports) > 0 {
			ingressPorts := getSvcPublicPorts(svcName, s)
			if len(ingressPorts) == 1 {
				publicSvcsMap[svcName] = true
			} else if len(ingressPorts) > 1 {
				for _, p := range ingressPorts {
					publicSvcsMap[fmt.Sprintf("%s-%d", svcName, p.ContainerPort)] = true
				}
			}
		}
	}
	for i := range iList {
		if _, ok := s.Endpoints[iList[i].GetName()]; ok {
			continue
		}
		if _, ok := publicSvcsMap[iList[i].GetName()]; ok {
			continue
		}
		if iList[i].GetLabels()[model.StackEndpointNameLabel] == "" {
			// ingress created with "public"
			continue
		}
		if err := iClient.Destroy(ctx, iList[i].GetName(), iList[i].GetNamespace()); err != nil {
			return fmt.Errorf("error destroying ingress '%s': %w", iList[i].GetName(), err)
		}
		oktetoLog.Success("Endpoint '%s' destroyed", iList[i].GetName())
	}
	return nil
}
