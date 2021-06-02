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
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/jobs"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/registry"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Deploy deploys a stack
func Deploy(ctx context.Context, s *model.Stack, forceBuild, wait, noCache bool) error {
	if s.Namespace == "" {
		s.Namespace = client.GetContextNamespace("")
	}

	c, _, err := client.GetLocal()
	if err != nil {
		return err
	}

	if err := translate(ctx, s, forceBuild, noCache); err != nil {
		return err
	}

	cfg := translateConfigMap(s)
	output := fmt.Sprintf("Deploying stack '%s'...", s.Name)
	cfg.Data[statusField] = progressingStatus
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	if err := configmaps.Deploy(ctx, cfg, s.Namespace, c); err != nil {
		return err
	}

	err = deploy(ctx, s, wait, c)
	if err != nil {
		output = fmt.Sprintf("%s\nStack '%s' deployment failed: %s", output, s.Name, err.Error())
		cfg.Data[statusField] = errorStatus
		cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	} else {
		output = fmt.Sprintf("%s\nStack '%s' successfully deployed", output, s.Name)
		cfg.Data[statusField] = deployedStatus
		cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	}

	if err := configmaps.Deploy(ctx, cfg, s.Namespace, c); err != nil {
		return err
	}

	return err
}

func deploy(ctx context.Context, s *model.Stack, wait bool, c *kubernetes.Clientset) error {
	DisplayWarnings(s)
	spinner := utils.NewSpinner(fmt.Sprintf("Deploying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	addHiddenExposedPorts(ctx, s)

	for name := range s.Services {
		if len(s.Services[name].Ports) > 0 {
			svcK8s := translateService(name, s)
			if err := services.Deploy(ctx, svcK8s, c); err != nil {
				return err
			}
		}
	}

	for name := range s.Volumes {
		if err := deployVolume(ctx, name, s, c); err != nil {
			return err
		}
		spinner.Stop()
		log.Success("Created volume '%s'", name)
		spinner.Start()
	}

	for name := range s.Services {
		if s.Services[name].RestartPolicy != apiv1.RestartPolicyAlways {
			if err := deployJob(ctx, name, s, c); err != nil {
				return err
			}
		} else if len(s.Services[name].Volumes) == 0 {
			if err := deployDeployment(ctx, name, s, c); err != nil {
				return err
			}
		} else {
			if err := deployStatefulSet(ctx, name, s, c); err != nil {
				return err
			}
		}
		spinner.Stop()
		log.Success("Deployed service '%s'", name)
		spinner.Start()
	}

	iClient, err := ingresses.GetClient(ctx, c)
	if err != nil {
		return fmt.Errorf("error getting ingress client: %s", err.Error())
	}
	for name := range s.Endpoints {
		if err := deployIngress(ctx, name, s, iClient); err != nil {
			return err
		}
		spinner.Stop()
		log.Success("Created endpoint '%s'", name)
		spinner.Start()
	}

	if err := destroyServicesNotInStack(ctx, spinner, s, c); err != nil {
		return err
	}

	if !wait {
		return nil
	}

	spinner.Update("Waiting for services to be ready...")
	return waitForPodsToBeRunning(ctx, s, c)
}

func deployDeployment(ctx context.Context, svcName string, s *model.Stack, c *kubernetes.Clientset) error {
	d := translateDeployment(svcName, s)
	old, err := c.AppsV1().Deployments(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting deployment of service '%s': %s", svcName, err.Error())
	}
	isNewDeployment := old.Name == ""
	if !isNewDeployment {
		if old.Labels[okLabels.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the deployment '%s' was running before deploying your stack", svcName)
		}
		if old.Labels[okLabels.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the deployment '%s' belongs to the stack '%s'", svcName, old.Labels[okLabels.StackNameLabel])
		}
		if deployments.IsDevModeOn(old) {
			deployments.RestoreDevModeFrom(d, old)
		}
	}

	if isNewDeployment {
		if err := deployments.Create(ctx, d, c); err != nil {
			return fmt.Errorf("error creating deployment of service '%s': %s", svcName, err.Error())
		}
	} else {
		if err := deployments.Update(ctx, d, c); err != nil {
			return fmt.Errorf("error updating deployment of service '%s': %s", svcName, err.Error())
		}
	}

	return nil
}

func deployStatefulSet(ctx context.Context, svcName string, s *model.Stack, c *kubernetes.Clientset) error {
	sfs := translateStatefulSet(svcName, s)
	old, err := c.AppsV1().StatefulSets(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting statefulset of service '%s': %s", svcName, err.Error())
	}
	if old.Name == "" {
		if err := statefulsets.Create(ctx, sfs, c); err != nil {
			return fmt.Errorf("error creating statefulset of service '%s': %s", svcName, err.Error())
		}
	} else {
		if old.Labels[okLabels.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the statefulset '%s' was running before deploying your stack", svcName)
		}
		if old.Labels[okLabels.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the statefulset '%s' belongs to the stack '%s'", svcName, old.Labels[okLabels.StackNameLabel])
		}
		if v, ok := old.Labels[okLabels.DeployedByLabel]; ok {
			sfs.Labels[okLabels.DeployedByLabel] = v
		}
		if err := statefulsets.Update(ctx, sfs, c); err != nil {
			if !strings.Contains(err.Error(), "Forbidden: updates to statefulset spec") {
				return fmt.Errorf("error updating statefulset of service '%s': %s", svcName, err.Error())
			}
			if err := statefulsets.Destroy(ctx, sfs.Name, sfs.Namespace, c); err != nil {
				return fmt.Errorf("error updating statefulset of service '%s': %s", svcName, err.Error())
			}
			if err := statefulsets.Create(ctx, sfs, c); err != nil {
				return fmt.Errorf("error updating statefulset of service '%s': %s", svcName, err.Error())
			}
		}
	}
	return nil
}

func deployJob(ctx context.Context, svcName string, s *model.Stack, c *kubernetes.Clientset) error {
	job := translateJob(svcName, s)
	old, err := c.BatchV1().Jobs(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting job of service '%s': %s", svcName, err.Error())
	}
	isNewJob := old.Name == ""
	if !isNewJob {
		if old.Labels[okLabels.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the job '%s' was running before deploying your stack", svcName)
		}
		if old.Labels[okLabels.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the job '%s' belongs to the stack '%s'", svcName, old.Labels[okLabels.StackNameLabel])
		}
	}

	if isNewJob {
		if err := jobs.Create(ctx, job, c); err != nil {
			return fmt.Errorf("error creating job of service '%s': %s", svcName, err.Error())
		}
	} else {
		if err := jobs.Update(ctx, job, c); err != nil {
			return fmt.Errorf("error updating job of service '%s': %s", svcName, err.Error())
		}
	}
	return nil
}

func deployVolume(ctx context.Context, volumeName string, s *model.Stack, c *kubernetes.Clientset) error {
	pvc := translatePersistentVolumeClaim(volumeName, s)

	old, err := c.CoreV1().PersistentVolumeClaims(s.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting volume '%s': %s", pvc.Name, err.Error())
	}
	if old.Name == "" {
		if err := volumes.Create(ctx, &pvc, c); err != nil {
			return fmt.Errorf("error creating volume '%s': %s", pvc.Name, err.Error())
		}
	} else {
		if old.Labels[okLabels.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the volume '%s' was running before deploying your stack", pvc.Name)
		}
		if old.Labels[okLabels.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the volume '%s' belongs to the stack '%s'", pvc.Name, old.Labels[okLabels.StackNameLabel])
		}

		old.Spec.Resources.Requests["storage"] = pvc.Spec.Resources.Requests["storage"]
		for key, value := range pvc.Labels {
			old.Labels[key] = value
		}
		for key, value := range pvc.Annotations {
			old.Annotations[key] = value
		}
		if pvc.Spec.StorageClassName != nil {
			old.Spec.StorageClassName = pvc.Spec.StorageClassName
		}

		if err := volumes.Update(ctx, old, c); err != nil {
			if strings.Contains(err.Error(), "spec.resources.requests.storage: Forbidden: field can not be less than previous value") {
				return fmt.Errorf("error updating volume '%s': Volume size can not be less than previous value", old.Name)
			}
			return fmt.Errorf("error updating volume '%s': %s", old.Name, err.Error())
		}
	}
	return nil
}

func deployIngress(ctx context.Context, ingressName string, s *model.Stack, c *ingresses.Client) error {
	iModel := &ingresses.Ingress{
		V1:      translateIngressV1(ingressName, s),
		V1Beta1: translateIngressV1Beta1(ingressName, s),
	}
	old, err := c.Get(ctx, ingressName, s.Namespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting ingress '%s': %s", ingressName, err.Error())
		}
		return c.Create(ctx, iModel)
	}

	if old.GetLabels()[okLabels.StackNameLabel] == "" {
		return fmt.Errorf("name collision: the ingress '%s' was running before deploying your stack", ingressName)
	}

	if old.GetLabels()[okLabels.StackNameLabel] != s.Name {
		return fmt.Errorf("name collision: the endpoint '%s' belongs to the stack '%s'", ingressName, old.GetLabels()[okLabels.StackNameLabel])
	}

	return c.Update(ctx, iModel)
}

func waitForPodsToBeRunning(ctx context.Context, s *model.Stack, c *kubernetes.Clientset) error {
	var numPods int32 = 0
	for _, svc := range s.Services {
		numPods += svc.Replicas
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.Now().Add(300 * time.Second)

	selector := map[string]string{okLabels.StackNameLabel: s.Name}
	for time.Now().Before(timeout) {
		<-ticker.C
		pendingPods := numPods
		podList, err := pods.ListBySelector(ctx, s.Namespace, selector, c)
		if err != nil {
			return err
		}
		for i := range podList {
			if podList[i].Status.Phase == apiv1.PodRunning || podList[i].Status.Phase == apiv1.PodSucceeded {
				pendingPods--
			}
			if podList[i].Status.Phase == apiv1.PodFailed {
				return fmt.Errorf("Service '%s' has failed. Please check for errors and try again", podList[i].Labels[okLabels.StackServiceNameLabel])
			}
		}
		if pendingPods == 0 {
			return nil
		}
	}
	return fmt.Errorf("kubernetes is taking too long to create your stack. Please check for errors and try again")
}

func DisplayWarnings(s *model.Stack) {
	DisplayNotSupportedFieldsWarnings(model.GroupWarningsBySvc(s.Warnings.NotSupportedFields))
	DisplayVolumeMountWarnings(s.Warnings.VolumeMountWarnings)
	DisplaySanitizedServicesWarnings(s.Warnings.SanitizedServices)
}

func DisplayNotSupportedFieldsWarnings(warnings []string) {
	if len(warnings) > 0 {
		if len(warnings) == 1 {
			log.Warning("'%s' field is not currently supported and will be ignored.", warnings[0])
		} else {
			notSupportedFields := strings.Join(model.GroupWarningsBySvc(warnings), "\n  - ")
			log.Warning("The following fields are not currently supported and will be ignored: \n  - %s", notSupportedFields)
		}
		log.Yellow("Help us to decide which fields to implement next by filing an issue in https://github.com/okteto/okteto/issues/new")
	}
}

func DisplayVolumeMountWarnings(warnings []string) {
	for _, warning := range warnings {
		log.Warning(warning)
	}
}

func DisplaySanitizedServicesWarnings(previousToNewNameMap map[string]string) {
	for previousName, newName := range previousToNewNameMap {
		log.Warning("Service '%s' has been sanitized into '%s'. This may affect discovery service.", previousName, newName)
	}
}

func addHiddenExposedPorts(ctx context.Context, s *model.Stack) {
	for _, svc := range s.Services {
		if svc.Image != "" {
			exposedPorts := registry.GetHiddenExposePorts(ctx, s.Namespace, svc.Image)
			for _, port := range exposedPorts {
				if !model.IsAlreadyAdded(port, svc.Ports) {
					svc.Ports = append(svc.Ports, port)
				}
			}
		}
	}

}
