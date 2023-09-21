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

package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/linguist"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	componentLabels []string = []string{"app.kubernetes.io/component", "component", "app"}
)

const (
	defaultWorkdirPath = "/okteto"
)

// GetDevDefaultsFromImage sets dev defaults from a image
func GetDevDefaultsFromImage(app apps.App) (registry.ImageMetadata, error) {
	image := app.PodSpec().Containers[0].Image
	reg := registry.NewOktetoRegistry(okteto.Config{})
	return reg.GetImageMetadata(image)
}

// SetImage sets dev defaults from a running app
func SetImage(dev *model.Dev, language, path string) {
	language = linguist.NormalizeLanguage(language)
	updateImageFromPod := false
	switch language {
	case linguist.Javascript:
		if path != "" {
			updateImageFromPod = !doesItHaveWebpackAsDependency(path)
		}
	case linguist.Python, linguist.Ruby, linguist.Php:
		updateImageFromPod = true
	}
	if updateImageFromPod {
		dev.Image = nil
	}
}

// SetDevDefaultsFromApp sets dev defaults from a running app
func SetDevDefaultsFromApp(ctx context.Context, dev *model.Dev, app apps.App, container, language, path string) error {
	c, config, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	setAnnotationsFromApp(dev, app)
	setNameAndLabelsFromApp(dev, app)

	pod, err := getRunningPod(ctx, app, container, c)
	if err != nil {
		return err
	}

	updateImageFromPod := true
	if dev.Image != nil {
		language = linguist.NormalizeLanguage(language)
		updateImageFromPod = false
		switch language {
		case linguist.Javascript:
			if path != "" {
				updateImageFromPod = !doesItHaveWebpackAsDependency(path)
			} else {
				updateImageFromPod = pods.HasPackageJson(ctx, pod, container, config, c)
			}

		case linguist.Python, linguist.Ruby, linguist.Php:
			updateImageFromPod = true
		}
	}

	if updateImageFromPod {
		dev.Image = nil
		dev.SecurityContext = getSecurityContextFromPod(ctx, pod, container, config, c)
		if dev.Workdir == "" {
			dev.Sync.Folders[0].RemotePath = getWorkdirFromPod(ctx, dev, pod, container, config, c)
		}
		if len(dev.Command.Values) == 0 {
			dev.Command.Values = getCommandFromPod(ctx, pod, container, config, c)
		}

	}

	if !okteto.IsOkteto() {
		setResourcesFromPod(dev, pod, container)
	}

	if err := setForwardsFromPod(ctx, dev, pod, c); err != nil {
		return err
	}

	discardReverseIfCollision(dev)
	return nil
}

func getRunningPod(ctx context.Context, app apps.App, container string, c kubernetes.Interface) (*apiv1.Pod, error) {
	pod, err := app.GetRunningPod(ctx, c)
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			return nil, fmt.Errorf("%s '%s': no pod is running", app.Kind(), app.ObjectMeta().Name)
		}
		return nil, err
	}

	if pod.Status.Phase != apiv1.PodRunning {
		return nil, fmt.Errorf("%s '%s': no pod is running", app.Kind(), app.ObjectMeta().Name)
	}
	for _, containerstatus := range pod.Status.ContainerStatuses {
		if containerstatus.Name == container && containerstatus.State.Running == nil {
			return nil, fmt.Errorf("%s '%s': no pod is running", app.Kind(), app.ObjectMeta().Name)
		}
	}
	return pod, nil
}

func getSecurityContextFromPod(ctx context.Context, pod *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) *model.SecurityContext {
	userID, err := pods.GetUserByPod(ctx, pod, container, config, c)
	if err != nil {
		oktetoLog.Infof("error getting user of the deployment: %s", err)
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
		oktetoLog.Infof("error getting workdir of the deployment: %s", err)
		if dev.Workdir == "/" {
			return defaultWorkdirPath
		}
		return dev.Workdir
	}
	if workdir == "/" {
		return defaultWorkdirPath
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
			forward.Forward{
				Local:  localPort,
				Remote: port,
			},
		)
	}
	return nil
}

// discardReverseIfCollision discards the reverse forward port if there is a collision with a forwarded port
func discardReverseIfCollision(dev *model.Dev) {
	seenPorts := map[int]bool{}
	for _, forward := range dev.Forward {
		seenPorts[forward.Local] = true
	}
	for i, reverse := range dev.Reverse {
		if seenPorts[reverse.Local] {
			dev.Reverse = append(dev.Reverse[:i], dev.Reverse[i+1:]...)
		}
	}
}

func setNameAndLabelsFromApp(dev *model.Dev, app apps.App) {
	for _, l := range componentLabels {
		component := app.ObjectMeta().Labels[l]
		if component == "" {
			continue
		}
		dev.Name = component
		dev.Selector = map[string]string{l: component}
		return
	}
	dev.Name = app.ObjectMeta().Name
}

func setAnnotationsFromApp(dev *model.Dev, app apps.App) {
	if v := app.ObjectMeta().Annotations[model.FluxAnnotation]; v != "" {
		dev.Metadata.Annotations = map[string]string{"fluxcd.io/ignore": "true"}
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

func doesItHaveWebpackAsDependency(path string) bool {
	packagePath := filepath.Join(path, "package.json")
	if _, err := os.Stat(packagePath); err != nil {
		oktetoLog.Infof("could not detect 'package.json' on path: %s", path)
		return false
	}

	b, err := os.ReadFile(packagePath)
	if err != nil {
		oktetoLog.Infof("could not read 'package.json' on '%s' because of: %s", path, err)
		return false
	}
	type dependencyPackage struct {
		Dependencies    map[string]string `json:"dependencies,omitempty"`
		DevDependencies map[string]string `json:"devDependencies,omitempty"`
	}
	dependencies := &dependencyPackage{}
	if err := json.Unmarshal(b, dependencies); err != nil {
		oktetoLog.Infof("could not unmarshal 'package.json' on '%s' because of: %s", path, err)
		return false
	}
	for key := range dependencies.Dependencies {
		if strings.Contains(key, "webpack") {
			return true
		}
	}
	for key := range dependencies.DevDependencies {
		if strings.Contains(key, "webpack") {
			return true
		}
	}
	return false
}
