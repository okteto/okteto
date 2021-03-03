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

package init

import (
	"context"
	"fmt"

	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

var (
	componentLabels []string = []string{"app.kubernetes.io/component", "component", "app"}
)

//SetDevDefaultsFromDeployment sets dev defaults from a running deployment
func SetDevDefaultsFromDeployment(ctx context.Context, dev *model.Dev, d *appsv1.Deployment, container string) error {
	c, _, err := k8Client.GetLocalWithContext(dev.Context)
	if err != nil {
		return err
	}

	setAnnotationsFromDeployment(dev, d)
	setNameAndLabelsFromDeployment(ctx, dev, d)

	pod, err := getRunningPod(ctx, d, container, c)
	if err != nil {
		return err
	}
	setResourcesFromPod(dev, pod, container)
	return setForwardsFromPod(ctx, dev, pod, c)
}

func getRunningPod(ctx context.Context, d *appsv1.Deployment, container string, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	rs, err := replicasets.GetReplicaSetByDeployment(ctx, d, "", c)
	if err != nil {
		return nil, err
	}
	pod, err := pods.GetPodByReplicaSet(ctx, rs, "", c)

	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, fmt.Errorf("no pod is running for deployment '%s'", d.Name)
	}

	if pod.Status.Phase != apiv1.PodRunning {
		return nil, fmt.Errorf("no pod is running for deployment '%s'", d.Name)
	}
	for _, containerstatus := range pod.Status.ContainerStatuses {
		if containerstatus.Name == container && containerstatus.State.Running == nil {
			return nil, fmt.Errorf("no pod is running for deployment '%s'", d.Name)
		}
	}
	return pod, nil
}

func setForwardsFromPod(ctx context.Context, dev *model.Dev, pod *apiv1.Pod, c *kubernetes.Clientset) error {
	ports, err := services.GetPortsByPod(ctx, pod, c)
	if err != nil {
		return err
	}
	seenPorts := map[int]bool{}
	for _, f := range dev.Forward {
		seenPorts[f.Local] = true
	}
	for _, port := range ports {
		localPort := port
		if port <= 1024 {
			localPort = port + 8000
		}
		for seenPorts[localPort] {
			localPort++
		}
		seenPorts[localPort] = true
		dev.Forward = append(
			dev.Forward,
			model.Forward{
				Local:  localPort,
				Remote: port,
			},
		)
	}
	return nil
}

func setNameAndLabelsFromDeployment(ctx context.Context, dev *model.Dev, d *appsv1.Deployment) {
	for _, l := range componentLabels {
		component := d.Labels[l]
		if component == "" {
			continue
		}
		dev.Name = component
		dev.Labels = map[string]string{l: component}
		return
	}
	dev.Name = d.Name
}

func setAnnotationsFromDeployment(dev *model.Dev, d *appsv1.Deployment) {
	if v := d.Annotations[okLabels.FluxAnnotation]; v != "" {
		dev.Annotations = map[string]string{"fluxcd.io/ignore": "true"}
	}
}

func setResourcesFromPod(dev *model.Dev, pod *apiv1.Pod, container string) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name != container {
			continue
		}
		if pod.Spec.Containers[i].Resources.Limits != nil {
			dev.Resources = model.ResourceRequirements{
				Limits: model.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("1"),
					apiv1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}
		}
		return
	}
}
