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
	"os"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/ingress"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

//Destroy destroys a stack
func Destroy(ctx context.Context, s *model.Stack, removeVolumes bool, timeout time.Duration) error {
	if s.Namespace == "" {
		s.Namespace = client.GetContextNamespace("")
	}

	c, _, _ := client.GetLocal()

	cfg := translateConfigMap(s)
	output := fmt.Sprintf("Destroying stack '%s'...", s.Name)
	cfg.Data[statusField] = destroyingStatus
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	if err := configmaps.Deploy(ctx, cfg, s.Namespace, c); err != nil {
		return err
	}

	err := destroy(ctx, s, removeVolumes, c, timeout)
	if err != nil {
		output = fmt.Sprintf("%s\nStack '%s' destruction failed: %s", output, s.Name, err.Error())
		cfg.Data[statusField] = errorStatus
		cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
		if err := configmaps.Deploy(ctx, cfg, s.Namespace, c); err != nil {
			return err
		}
	} else if err := configmaps.Destroy(ctx, cfg.Name, s.Namespace, c); err != nil {
		return err
	}
	return err
}

func destroy(ctx context.Context, s *model.Stack, removeVolumes bool, c *kubernetes.Clientset, timeout time.Duration) error {
	spinner := utils.NewSpinner(fmt.Sprintf("Destroying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	if err := destroyHelmRelease(ctx, spinner, s); err != nil {
		return err
	}

	s.Services = nil
	if err := destroyServicesNotInStack(ctx, spinner, s, c); err != nil {
		return err
	}

	spinner.Update("Waiting for services to be destroyed...")
	if err := waitForPodsToBeDestroyed(ctx, s, c); err != nil {
		return err
	}

	if removeVolumes {
		spinner.Update("Destroying volumes...")
		if err := destroyStackVolumes(ctx, spinner, s, c, timeout); err != nil {
			return err
		}
	}

	return configmaps.Destroy(ctx, s.GetConfigMapName(), s.Namespace, c)
}

func helmReleaseExist(c *action.List, name string) (bool, error) {
	c.AllNamespaces = false
	results, err := c.Run()
	if err != nil {
		return false, err
	}
	for _, release := range results {
		if release.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func destroyHelmRelease(ctx context.Context, spinner *utils.Spinner, s *model.Stack) error {
	settings := cli.New()
	settings.KubeContext = os.Getenv(client.OktetoContextVariableName)

	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(settings.RESTClientGetter(), s.Namespace, helmDriver, func(format string, v ...interface{}) {
		message := strings.TrimSuffix(fmt.Sprintf(format, v...), "\n")
		spinner.Update(fmt.Sprintf("%s...", message))
	}); err != nil {
		return fmt.Errorf("error initializing stack client: %s", err)
	}

	exists, err := helmReleaseExist(action.NewList(actionConfig), s.Name)
	if err != nil {
		return fmt.Errorf("error listing stacks: %s", err)
	}
	if exists {
		uClient := action.NewUninstall(actionConfig)
		if _, err := uClient.Run(s.Name); err != nil {
			return fmt.Errorf("error destroying stack '%s': %s", s.Name, err.Error())
		}
	}
	return nil
}

func destroyServicesNotInStack(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c *kubernetes.Clientset) error {
	dList, err := deployments.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range dList {
		if _, ok := s.Services[dList[i].Name]; ok {
			continue
		}
		if err := deployments.Destroy(ctx, dList[i].Name, dList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying deployment of service '%s': %s", dList[i].Name, err)
		}
		if err := services.Destroy(ctx, dList[i].Name, dList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %s", dList[i].Name, err)
		}
		spinner.Stop()
		log.Success("Destroyed service '%s'", dList[i].Name)
		spinner.Start()
	}

	sfsList, err := statefulsets.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range sfsList {
		if _, ok := s.Services[sfsList[i].Name]; ok {
			continue
		}
		if err := statefulsets.Destroy(ctx, sfsList[i].Name, sfsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying statefulset of service '%s': %s", sfsList[i].Name, err)
		}
		if err := services.Destroy(ctx, sfsList[i].Name, sfsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %s", sfsList[i].Name, err)
		}
		spinner.Stop()
		log.Success("Destroyed service '%s'", sfsList[i].Name)
		spinner.Start()
	}

	pvcList, err := volumes.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range pvcList {
		for _, volume := range s.Volumes {
			if volume.Name == pvcList[i].Name {
				continue
			}
		}
		timeout, err := model.GetTimeout()
		if err != nil {
			return err
		}
		if err := volumes.Destroy(ctx, pvcList[i].Name, pvcList[i].Namespace, c, timeout); err != nil {
			return fmt.Errorf("error destroying persistent volume claim of service '%s': %s", pvcList[i].Name, err)
		}
		spinner.Stop()
		log.Success("Destroyed volume '%s'", sfsList[i].Name)
		spinner.Start()
	}

	ingressesList, err := ingress.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range ingressesList {
		if _, ok := s.Endpoints[ingressesList[i].Name]; ok {
			continue
		}
		if err := ingress.Destroy(ctx, ingressesList[i].Name, ingressesList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying ingress '%s': %s", ingressesList[i].Name, err)
		}
		spinner.Stop()
		log.Success("Destroyed endpoint '%s'", ingressesList[i].Name)
		spinner.Start()
	}

	return nil
}

func waitForPodsToBeDestroyed(ctx context.Context, s *model.Stack, c *kubernetes.Clientset) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.Now().Add(300 * time.Second)

	selector := map[string]string{okLabels.StackNameLabel: s.Name}
	for time.Now().Before(timeout) {
		<-ticker.C
		podList, err := pods.ListBySelector(ctx, s.Namespace, selector, c)
		if err != nil {
			return err
		}
		if len(podList) == 0 {
			return nil
		}
	}
	return fmt.Errorf("kubernetes is taking too long to destroy your stack. Please check for errors and try again")
}

func destroyStackVolumes(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c *kubernetes.Clientset, timeout time.Duration) error {
	vList, err := volumes.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for _, v := range vList {
		if v.Labels[okLabels.StackNameLabel] == s.Name {
			if err := volumes.Destroy(ctx, v.Name, v.Namespace, c, timeout); err != nil {
				return fmt.Errorf("error destroying volume '%s': %s", v.Name, err)
			}
			spinner.Stop()
			log.Success("Destroyed volume '%s'", v.Name)
			spinner.Start()
		}
	}
	return nil
}
