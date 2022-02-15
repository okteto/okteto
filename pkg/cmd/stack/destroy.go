// Copyright 2022 The Okteto Authors
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
	"time"

	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/jobs"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

// Destroy destroys a stack
func Destroy(ctx context.Context, s *model.Stack, removeVolumes bool, timeout time.Duration) error {
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to load your local Kubeconfig: %s", err)
	}

	cfg := translateConfigMap(s)
	output := fmt.Sprintf("Destroying compose '%s'...", s.Name)
	cfg.Data[statusField] = destroyingStatus
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	if err := configmaps.Deploy(ctx, cfg, s.Namespace, c); err != nil {
		return err
	}

	err = destroy(ctx, s, removeVolumes, c, timeout)
	if err != nil {
		output = fmt.Sprintf("%s\nCompose '%s' destruction failed: %s", output, s.Name, err.Error())
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
	spinner := utils.NewSpinner(fmt.Sprintf("Destroying compose '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		s.Services = nil
		s.Endpoints = nil
		if err := destroyServicesNotInStack(ctx, spinner, s, c); err != nil {
			exit <- err
			return
		}

		spinner.Update("Waiting for services to be destroyed...")
		if err := waitForPodsToBeDestroyed(ctx, s, c); err != nil {
			exit <- err
			return
		}
		if removeVolumes {
			spinner.Update("Destroying volumes...")
			if err := destroyStackVolumes(ctx, spinner, s, c, timeout); err != nil {
				exit <- err
				return
			}
		}
		exit <- configmaps.Destroy(ctx, model.GetStackConfigMapName(s.Name), s.Namespace, c)
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func destroyServicesNotInStack(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c kubernetes.Interface) error {
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

func destroyDeployments(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c kubernetes.Interface) error {
	dList, err := deployments.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range dList {
		if _, ok := s.Services[dList[i].Name]; ok && s.Services[dList[i].Name].IsDeployment() {
			continue
		}
		if err := deployments.Destroy(ctx, dList[i].Name, dList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying deployment of service '%s': %s", dList[i].Name, err)
		}
		if err := services.Destroy(ctx, dList[i].Name, dList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %s", dList[i].Name, err)
		}
		spinner.Stop()
		if _, ok := s.Services[dList[i].Name]; ok {
			oktetoLog.Success("Destroyed previous service '%s'", dList[i].Name)
		} else {
			oktetoLog.Success("Destroyed service '%s'", dList[i].Name)
		}
		spinner.Start()
	}
	return nil
}

func destroyStatefulsets(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c kubernetes.Interface) error {
	sfsList, err := statefulsets.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range sfsList {
		if _, ok := s.Services[sfsList[i].Name]; ok && s.Services[sfsList[i].Name].IsStatefulset() {
			continue
		}
		if err := statefulsets.Destroy(ctx, sfsList[i].Name, sfsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying statefulset of service '%s': %s", sfsList[i].Name, err)
		}
		if err := services.Destroy(ctx, sfsList[i].Name, sfsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %s", sfsList[i].Name, err)
		}
		spinner.Stop()
		if _, ok := s.Services[sfsList[i].Name]; ok {
			oktetoLog.Success("Destroyed previous service '%s'", sfsList[i].Name)
		} else {
			oktetoLog.Success("Destroyed service '%s'", sfsList[i].Name)
		}
		spinner.Start()
	}
	return nil
}
func destroyJobs(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c kubernetes.Interface) error {
	jobsList, err := jobs.List(ctx, s.Namespace, s.GetLabelSelector(), c)
	if err != nil {
		return err
	}
	for i := range jobsList {
		if _, ok := s.Services[jobsList[i].Name]; ok && s.Services[jobsList[i].Name].IsJob() {
			continue
		}
		if err := jobs.Destroy(ctx, jobsList[i].Name, jobsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying job of service '%s': %s", jobsList[i].Name, err)
		}
		if err := services.Destroy(ctx, jobsList[i].Name, jobsList[i].Namespace, c); err != nil {
			return fmt.Errorf("error destroying service '%s': %s", jobsList[i].Name, err)
		}
		spinner.Stop()
		if _, ok := s.Services[jobsList[i].Name]; ok {
			oktetoLog.Success("Destroyed previous service '%s'", jobsList[i].Name)
		} else {
			oktetoLog.Success("Destroyed service '%s'", jobsList[i].Name)
		}
		spinner.Start()
	}
	return nil
}

func destroyIngresses(ctx context.Context, spinner *utils.Spinner, s *model.Stack, c kubernetes.Interface) error {
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
		oktetoLog.Success("Destroyed endpoint '%s'", iList[i].GetName())
		spinner.Start()
	}
	return nil
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
			oktetoLog.Success("Destroyed volume '%s'", v.Name)
			spinner.Start()
		}
	}
	return nil
}
