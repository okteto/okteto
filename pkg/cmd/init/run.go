// Copyright 2021 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/linguist"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	componentLabels []string = []string{"app.kubernetes.io/component", "component", "app"}
)

// SetDevDefaultsFromDeployment sets dev defaults from a running deployment
func SetDevDefaultsFromApp(ctx context.Context, dev *model.Dev, app apps.App, container, language string) error {
	c, config, err := client.GetLocalWithContext(dev.Context)
	if err != nil {
		return err
	}

	pod, err := getRunningPod(ctx, app, container, c)
	if err != nil {
		return err
	}

	language = linguist.NormalizeLanguage(language)
	updateImageFromPod := false
	switch language {
	case linguist.Javascript:
		updateImageFromPod = pods.HasPackageJson(ctx, pod, container, config, c)
	case linguist.Python, linguist.Ruby, linguist.Php:
		updateImageFromPod = true
	}

	if updateImageFromPod {
		dev.Image = nil
		dev.SecurityContext = getSecurityContextFromPod(ctx, dev, pod, container, config, c)
		dev.Sync.Folders[0].RemotePath = getWorkdirFromPod(ctx, dev, pod, container, config, c)
		dev.Command.Values = getCommandFromPod(ctx, pod, container, config, c)
	}

	setAnnotationsFromApp(dev, app)
	setNameAndLabelsFromApp(dev, app)

	if okteto.GetClusterContext() != client.GetSessionContext("") {
		setResourcesFromPod(dev, pod, container)
	}

	return setForwardsFromPod(ctx, dev, pod, c)
}

func getRunningPod(ctx context.Context, app apps.App, container string, c kubernetes.Interface) (*apiv1.Pod, error) {
	pod, err := app.GetRunningPod(ctx, c)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("%s '%s': no pod is running", app.TypeMeta().Kind, app.ObjectMeta().Name)
		}
		return nil, err
	}

	if pod.Status.Phase != apiv1.PodRunning {
		return nil, fmt.Errorf("%s '%s': no pod is running", app.TypeMeta().Kind, app.ObjectMeta().Name)
	}
	for _, containerstatus := range pod.Status.ContainerStatuses {
		if containerstatus.Name == container && containerstatus.State.Running == nil {
			return nil, fmt.Errorf("%s '%s': no pod is running", app.TypeMeta().Kind, app.ObjectMeta().Name)
		}
	}
	return pod, nil
}

func getSecurityContextFromPod(ctx context.Context, dev *model.Dev, pod *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) *model.SecurityContext {
	userID, err := pods.GetUserByPod(ctx, pod, container, config, c)
	if err != nil {
		log.Infof("error getting user of the deployment: %s", err)
		return nil
	}
	if userID == 0 {
		return nil
	}
	return &model.SecurityContext{RunAsUser: &userID}
}

func getWorkdirFromPod(ctx context.Context, dev *model.Dev, pod *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) string {
	workdir, err := pods.GetWorkdirByPod(ctx, pod, container, config, c)
	if err != nil {
		log.Infof("error getting workdir of the deployment: %s", err)
		return dev.Workdir
	}
	return workdir
}

func getCommandFromPod(ctx context.Context, pod *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) []string {
	if pods.CheckIfBashIsAvailable(ctx, pod, container, config, c) {
		return []string{"bash"}
	}
	return []string{"sh"}
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

func setNameAndLabelsFromApp(dev *model.Dev, app apps.App) {
	for _, l := range componentLabels {
		component := app.ObjectMeta().Labels[l]
		if component == "" {
			continue
		}
		dev.Name = component
		dev.Labels = map[string]string{l: component}
		return
	}
	dev.Name = app.ObjectMeta().Name
}

func setAnnotationsFromApp(dev *model.Dev, app apps.App) {
	if v := app.ObjectMeta().Annotations[model.FluxAnnotation]; v != "" {
		dev.Annotations = map[string]string{"fluxcd.io/ignore": "true"}
	}
}

func setResourcesFromPod(dev *model.Dev, pod *apiv1.Pod, container string) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name != container {
			continue
		}
		if pod.Spec.Containers[i].Resources.Limits != nil {
			cpuLimits := pod.Spec.Containers[i].Resources.Limits[apiv1.ResourceCPU]
			if cpuLimits.Cmp(resource.MustParse("1")) < 0 {
				cpuLimits = resource.MustParse("1")
			}
			memoryLimits := pod.Spec.Containers[i].Resources.Limits[apiv1.ResourceMemory]
			if memoryLimits.Cmp(resource.MustParse("3Gi")) < 0 {
				memoryLimits = resource.MustParse("3Gi")
			}
			dev.Resources = model.ResourceRequirements{
				Limits: model.ResourceList{
					apiv1.ResourceCPU:    cpuLimits,
					apiv1.ResourceMemory: memoryLimits,
				},
			}
		}
		return
	}
}
