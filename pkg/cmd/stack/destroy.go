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

package stack

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/jobs"
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

// Destroy destroys a stack
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		if err := destroyHelmRelease(ctx, spinner, s); err != nil {
			exit <- err
		}
		s.Services = nil
		s.Endpoints = nil
		if err := destroyServicesNotInStack(ctx, spinner, s, c); err != nil {
			exit <- err
		}

		spinner.Update("Waiting for services to be destroyed...")
		if err := waitForPodsToBeDestroyed(ctx, s, c); err != nil {
			exit <- err
		}
		if removeVolumes {
			spinner.Update("Destroying volumes...")
			if err := destroyStackVolumes(ctx, spinner, s, c, timeout); err != nil {
				exit <- err
			}
		}
		exit <- configmaps.Destroy(ctx, model.GetStackConfigMapName(s.Name), s.Namespace, c)
	}()

	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		os.Exit(130)
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
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
	if err := destroyDeployments(ctx, spinner, s, c); err != nil {
		return err
	}

	if err := destroyStatefulsets(ctx, spinner, s, c); err != nil {
		return err
	}

	if err := destroyJobs(ctx, spinner, s, c); err != nil {
		return err
	}

	err := destroyIngresses(ctx, spinner, s, c)
	if err != nil {
		return err
	}

	return nil
}

func destroyDeployments(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c *kubernetes.Clientset) error {
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
	return nil
}

func destroyStatefulsets(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c *kubernetes.Clientset) error {
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
	return nil
}
func destroyJobs(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c *kubernetes.Clientset) error {
	jobsList, err := jobs.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range jobsList {
		if _, ok := s.Services[jobsList[i].Name]; ok {
			continue
		}
		if err := jobs.Destroy(ctx, jobsList[i].Name, jobsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying job of service '%s': %s", jobsList[i].Name, err)
		}
		if err := services.Destroy(ctx, jobsList[i].Name, jobsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %s", jobsList[i].Name, err)
		}
		spinner.Stop()
		log.Success("Destroyed service '%s'", jobsList[i].Name)
		spinner.Start()
	}
	return nil
}

func destroyIngresses(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c *kubernetes.Clientset) error {
	iClient, err := ingresses.GetClient(ctx, c)
	if err != nil {
		return fmt.Errorf("error getting ingress client: %s", err.Error())
	}

	iList, err := iClient.List(ctx, s.Namespace, s.GetLabelSelector())
	if err != nil {
		return err
	}
	for i := range iList {
		if _, ok := s.Endpoints[iList[i].GetName()]; ok {
			continue
		}
		if iList[i].GetLabels()[model.StackEndpointNameLabel] == "" {
			//ingress created with "public"
			continue
		}
		if err := iClient.Destroy(ctx, iList[i].GetName(), iList[i].GetNamespace()); err != nil {
			return fmt.Errorf("error destroying ingress '%s': %s", iList[i].GetName(), err)
		}
		spinner.Stop()
		log.Success("Destroyed endpoint '%s'", iList[i].GetName())
		spinner.Start()
	}
	return nil
}

func isRemovable(s *model.Stack, pvcName string) bool {
	isInList := false
	for volumeName := range s.Volumes {
		if volumeName == pvcName {
			isInList = true
			break
		}
		if isAutocreatedVolume(s, pvcName) {
			isInList = true
			break
		}
	}
	return isInList
}

func isAutocreatedVolume(s *model.Stack, volumeName string) bool {
	if strings.Contains(volumeName, "-") {
		splitted := strings.Split(volumeName, "-")
		if len(splitted) == 3 {
			svcName := splitted[1]
			volumeIdx, err := strconv.Atoi(splitted[2])
			if err != nil {
				return false
			}
			if svc, ok := s.Services[svcName]; ok {
				i := 0
				for _, volume := range svc.Volumes {
					if volume.LocalPath == "" {
						if volumeIdx == i {
							return true
						}
						i++
					}
				}
			}
		}
	}
	return false
}

func waitForPodsToBeDestroyed(ctx context.Context, s *model.Stack, c *kubernetes.Clientset) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.Now().Add(300 * time.Second)

	selector := map[string]string{model.StackNameLabel: s.Name}
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
		if v.Labels[model.StackNameLabel] == s.Name {
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
