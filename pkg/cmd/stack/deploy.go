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
	"github.com/okteto/okteto/pkg/k8s/ingress"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy deploys a stack
func Deploy(ctx context.Context, s *model.Stack, forceBuild, wait, noCache bool) error {
	if s.Namespace == "" {
		s.Namespace = client.GetContextNamespace("")
	}

	c, _, err := client.GetLocal()
	if err != nil {
		return err
	}

	cfg := translateConfigMap(s)
	output := fmt.Sprintf("Deploying stack '%s'...", s.Name)
	cfg.Data[statusField] = progressingStatus
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	if err := configmaps.Deploy(ctx, cfg, s.Namespace, c); err != nil {
		return err
	}

	err = deploy(ctx, s, forceBuild, wait, noCache, c)
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

func deploy(ctx context.Context, s *model.Stack, forceBuild, wait, noCache bool, c *kubernetes.Clientset) error {

	if err := translate(ctx, s, forceBuild, noCache); err != nil {
		return err
	}

	spinner := utils.NewSpinner(fmt.Sprintf("Deploying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	for name := range s.Services {
		if len(s.Services[name].Volumes) == 0 {
			if err := deployDeployment(ctx, name, s, c); err != nil {
				return err
			}
		} else {
			if err := deployStatefulSet(ctx, name, s, c); err != nil {
				return err
			}
		}
		if len(s.Services[name].Ports) > 0 {
			svcK8s := translateService(name, s)
			if err := services.Create(ctx, svcK8s, c); err != nil {
				return err
			}
		}
		spinner.Stop()
		log.Success("Deployed service '%s'", name)
		spinner.Start()
	}

	if err := destroyServicesNotInStack(ctx, spinner, s, c); err != nil {
		return err
	}

	for name := range s.Endpoints {
		if err := deployIngress(ctx, name, s, c); err != nil {
			return err
		}
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
		if d.Labels[okLabels.StackNameLabel] != old.Labels[okLabels.StackNameLabel] {
			return fmt.Errorf("name collision: the deployment '%s' belongs to the stack '%s'", svcName, old.Labels[okLabels.StackNameLabel])
		}
		if deployments.IsDevModeOn(old) {
			deployments.RestoreDevModeFrom(d, old)
		}
	}
	if err := deployments.Deploy(ctx, d, isNewDeployment, c); err != nil {
		if isNewDeployment {
			return fmt.Errorf("error creating deployment of service '%s': %s", svcName, err.Error())
		}
		return fmt.Errorf("error updating deployment of service '%s': %s", svcName, err.Error())
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
		if sfs.Labels[okLabels.StackNameLabel] != old.Labels[okLabels.StackNameLabel] {
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

func deployIngress(ctx context.Context, ingressName string, s *model.Stack, c *kubernetes.Clientset) error {
	ingressK8s := translateIngress(ingressName, s)
	old, err := c.ExtensionsV1beta1().Ingresses(s.Namespace).Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting ingress '%s': %s", ingressName, err.Error())
	}
	isNewIngress := old.Name == ""
	if !isNewIngress {
		if old.Labels[okLabels.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the ingress '%s' was running before deploying your stack", ingressName)
		}
		if ingressK8s.Labels[okLabels.StackNameLabel] != old.Labels[okLabels.StackNameLabel] {
			return fmt.Errorf("name collision: the ingress '%s' belongs to the stack '%s'", ingressName, old.Labels[okLabels.StackNameLabel])
		}
		ingress.Update(ctx, ingressK8s, c)
	} else {
		if err := ingress.Create(ctx, ingressK8s, c); err != nil {
			return err
		}
	}
	return nil
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
			if podList[i].Status.Phase == apiv1.PodRunning {
				pendingPods--
			}
		}
		if pendingPods == 0 {
			return nil
		}
	}
	return fmt.Errorf("kubernetes is taking too long to create your stack. Please check for errors and try again")
}
