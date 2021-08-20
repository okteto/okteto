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
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/jobs"
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
	"k8s.io/client-go/rest"
)

// Deploy deploys a stack
func Deploy(ctx context.Context, s *model.Stack, forceBuild, wait, noCache bool, timeout time.Duration) error {
	if s.Namespace == "" {
		s.Namespace = client.GetContextNamespace("")
	}

	c, config, err := client.GetLocal()
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

	err = deploy(ctx, s, wait, c, config, timeout)
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

func deploy(ctx context.Context, s *model.Stack, wait bool, c *kubernetes.Clientset, config *rest.Config, timeout time.Duration) error {
	DisplayWarnings(s)
	spinner := utils.NewSpinner(fmt.Sprintf("Deploying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		addHiddenExposedPortsToStack(ctx, s)

		for name := range s.Services {
			if len(s.Services[name].Ports) > 0 {
				svcK8s := translateService(name, s)
				if err := services.Deploy(ctx, svcK8s, c); err != nil {
					exit <- err
				}
			}
		}

		for name := range s.Volumes {
			if err := deployVolume(ctx, name, s, c); err != nil {
				exit <- err
			}
			spinner.Stop()
			log.Success("Created volume '%s'", name)
			spinner.Start()
		}

		if err := deployServices(ctx, s, c, config, spinner, timeout); err != nil {
			exit <- err
		}

		iClient, err := ingresses.GetClient(ctx, c)
		if err != nil {
			exit <- fmt.Errorf("error getting ingress client: %s", err.Error())
		}
		for name := range s.Endpoints {
			if err := deployIngress(ctx, name, s, iClient); err != nil {
				exit <- err
			}
			spinner.Stop()
			log.Success("Created endpoint '%s'", name)
			spinner.Start()
		}

		if err := destroyServicesNotInStack(ctx, spinner, s, c); err != nil {
			exit <- err
		}

		if !wait {
			exit <- nil
		}

		spinner.Update("Waiting for services to be ready...")
		exit <- waitForPodsToBeRunning(ctx, s, c)
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

func deployServices(ctx context.Context, stack *model.Stack, k8sClient *kubernetes.Clientset, config *rest.Config, spinner *utils.Spinner, timeout time.Duration) error {
	deployedSvcs := make(map[string]bool)
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(timeout)

	for {
		select {
		case <-to.C:
			return fmt.Errorf("stack '%s' didn't finish after %s", stack.Name, timeout.String())
		case <-t.C:
			for len(deployedSvcs) != len(stack.Services) {
				for svcName := range stack.Services {
					if deployedSvcs[svcName] {
						continue
					}

					if !canSvcBeDeployed(ctx, stack, svcName, k8sClient, config) {
						if failedJobs := getDependingFailedJobs(ctx, stack, svcName, k8sClient, config); len(failedJobs) > 0 {
							if len(failedJobs) == 1 {
								return fmt.Errorf("Service '%s' dependency '%s' failed", svcName, failedJobs[0])
							}
							return fmt.Errorf("Service '%s' dependencies '%s' failed", svcName, strings.Join(failedJobs, ", "))
						}
						if failedServices := getServicesWithFailedProbes(ctx, stack, svcName, k8sClient, config); len(failedServices) > 0 {
							for key, value := range failedServices {
								return fmt.Errorf("Service '%s' has failed his healthcheck probes: %s", key, value)
							}
						}
						continue
					}
					spinner.Update(fmt.Sprintf("Deploying service '%s'...", svcName))
					err := deploySvc(ctx, stack, svcName, k8sClient, spinner)
					if err != nil {
						return err
					}
					deployedSvcs[svcName] = true
					spinner.Update("Waiting for services to be ready...")
				}
			}
			return nil
		}
	}
}

func deploySvc(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, spinner *utils.Spinner) error {
	if stack.Services[svcName].IsJob() {
		if err := deployJob(ctx, svcName, stack, client); err != nil {
			return err
		}
	} else if len(stack.Services[svcName].Volumes) == 0 {
		if err := deployDeployment(ctx, svcName, stack, client); err != nil {
			return err
		}
	} else {
		if err := deployStatefulSet(ctx, svcName, stack, client); err != nil {
			return err
		}
	}
	spinner.Stop()
	log.Success("Deployed service '%s'", svcName)
	spinner.Start()
	return nil
}

func canSvcBeDeployed(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, config *rest.Config) bool {
	for dependentSvc, condition := range stack.Services[svcName].DependsOn {
		if !isSvcReady(ctx, stack, dependentSvc, condition, client, config) {
			return false
		}
	}
	return true
}

func getServicesWithFailedProbes(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, config *rest.Config) map[string]string {
	svc := stack.Services[svcName]
	dependingServices := make([]string, 0)
	for dependingSvc, condition := range svc.DependsOn {
		if stack.Services[dependingSvc].Healtcheck != nil && condition.Condition == model.DependsOnServiceHealthy {
			dependingServices = append(dependingServices, dependingSvc)
		}
	}
	failedServices := make(map[string]string)
	for _, dependingSvc := range dependingServices {

		if healthcheckFailure := pods.GetHealthcheckFailure(ctx, stack.Namespace, dependingSvc, stack.Name, client); healthcheckFailure != "" {
			failedServices[dependingSvc] = healthcheckFailure
		}
	}
	return failedServices
}

func getDependingFailedJobs(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, config *rest.Config) []string {
	svc := stack.Services[svcName]
	dependingJobs := make([]string, 0)
	for dependingSvc := range svc.DependsOn {
		if stack.Services[dependingSvc].IsJob() {
			dependingJobs = append(dependingJobs, dependingSvc)
		}
	}
	failedJobs := make([]string, 0)
	for _, jobName := range dependingJobs {
		if jobs.IsFailed(ctx, stack.Namespace, jobName, client) {
			failedJobs = append(failedJobs, jobName)
		}
	}
	return failedJobs
}

func isSvcReady(ctx context.Context, stack *model.Stack, dependentSvcName string, condition model.DependsOnConditionSpec, client kubernetes.Interface, config *rest.Config) bool {
	svc := stack.Services[dependentSvcName]

	switch condition.Condition {
	case model.DependsOnServiceRunning:
		return isSvcRunning(ctx, svc, stack.Namespace, dependentSvcName, client)
	case model.DependsOnServiceHealthy:
		return isSvcHealthy(ctx, stack, dependentSvcName, client, config)
	case model.DependsOnServiceCompleted:
		if jobs.IsSuccedded(ctx, stack.Namespace, dependentSvcName, client) {
			return true
		}
	}
	return false
}

func getPodName(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface) string {
	svcLabels := map[string]string{model.StackNameLabel: stack.Name, model.StackServiceNameLabel: svcName}

	p, err := pods.GetBySelector(ctx, stack.Namespace, svcLabels, client)
	if err != nil {
		return ""
	}
	return p.Name
}

func isSvcRunning(ctx context.Context, svc *model.Service, namespace, svcName string, client kubernetes.Interface) bool {

	switch {
	case svc.IsDeployment():
		if deployments.IsRunning(ctx, namespace, svcName, client) {
			return true
		}
	case svc.IsStatefulset():
		if statefulsets.IsRunning(ctx, namespace, svcName, client) {
			return true
		}
	case svc.IsJob():
		if jobs.IsRunning(ctx, namespace, svcName, client) {
			return true
		}
	}
	return false
}

func isSvcHealthy(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, config *rest.Config) bool {
	svc := stack.Services[svcName]
	if !isSvcRunning(ctx, svc, stack.Namespace, svcName, client) {
		return false
	}
	if svc.Healtcheck != nil {
		return true
	} else {
		return isAnyPortAvailable(ctx, svc, stack, svcName, client, config)
	}
}

func isAnyPortAvailable(ctx context.Context, svc *model.Service, stack *model.Stack, svcName string, client kubernetes.Interface, config *rest.Config) bool {
	forwarder := forward.NewPortForwardManager(ctx, model.Localhost, config, client, stack.Namespace)
	podName := getPodName(ctx, stack, svcName, client)
	if podName == "" {
		return false
	}
	portsToTest := make([]int, 0)
	for _, p := range svc.Ports {
		port, err := model.GetAvailablePort(model.Localhost)
		if err != nil {
			continue
		}
		portsToTest = append(portsToTest, port)
		if err := forwarder.Add(model.Forward{Local: port, Remote: int(p.ContainerPort)}); err != nil {
			continue
		}
	}
	forwarder.Start(podName, stack.Namespace)
	defer forwarder.Stop()
	for _, port := range portsToTest {
		url := fmt.Sprintf("%s:%d", model.Localhost, port)
		_, err := net.Dial("tcp", url)
		if err != nil {
			continue
		}
		return true
	}
	return false
}

func deployDeployment(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface) error {
	d := translateDeployment(svcName, s)
	old, err := c.AppsV1().Deployments(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting deployment of service '%s': %s", svcName, err.Error())
	}
	isNewDeployment := old == nil || old.Name == ""
	if !isNewDeployment {
		if old.Labels[model.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the deployment '%s' was running before deploying your stack", svcName)
		}
		if old.Labels[model.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the deployment '%s' belongs to the stack '%s'", svcName, old.Labels[model.StackNameLabel])
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

func deployStatefulSet(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface) error {
	sfs := translateStatefulSet(svcName, s)
	old, err := c.AppsV1().StatefulSets(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting statefulset of service '%s': %s", svcName, err.Error())
	}
	if old == nil || old.Name == "" {
		if err := statefulsets.Create(ctx, sfs, c); err != nil {
			return fmt.Errorf("error creating statefulset of service '%s': %s", svcName, err.Error())
		}
	} else {
		if old.Labels[model.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the statefulset '%s' was running before deploying your stack", svcName)
		}
		if old.Labels[model.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the statefulset '%s' belongs to the stack '%s'", svcName, old.Labels[model.StackNameLabel])
		}
		if v, ok := old.Labels[model.DeployedByLabel]; ok {
			sfs.Labels[model.DeployedByLabel] = v
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

func deployJob(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface) error {
	job := translateJob(svcName, s)
	old, err := c.BatchV1().Jobs(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting job of service '%s': %s", svcName, err.Error())
	}
	isNewJob := old == nil || old.Name == ""
	if !isNewJob {
		if old.Labels[model.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the job '%s' was running before deploying your stack", svcName)
		}
		if old.Labels[model.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the job '%s' belongs to the stack '%s'", svcName, old.Labels[model.StackNameLabel])
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

func deployVolume(ctx context.Context, volumeName string, s *model.Stack, c kubernetes.Interface) error {
	pvc := translatePersistentVolumeClaim(volumeName, s)

	old, err := c.CoreV1().PersistentVolumeClaims(s.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting volume '%s': %s", pvc.Name, err.Error())
	}
	if old == nil || old.Name == "" {
		if err := volumes.Create(ctx, &pvc, c); err != nil {
			return fmt.Errorf("error creating volume '%s': %s", pvc.Name, err.Error())
		}
	} else {
		if old.Labels[model.StackNameLabel] == "" {
			return fmt.Errorf("name collision: the volume '%s' was running before deploying your stack", pvc.Name)
		}
		if old.Labels[model.StackNameLabel] != s.Name {
			return fmt.Errorf("name collision: the volume '%s' belongs to the stack '%s'", pvc.Name, old.Labels[model.StackNameLabel])
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

	if old.GetLabels()[model.StackNameLabel] == "" {
		return fmt.Errorf("name collision: the ingress '%s' was running before deploying your stack", ingressName)
	}

	if old.GetLabels()[model.StackNameLabel] != s.Name {
		return fmt.Errorf("name collision: the endpoint '%s' belongs to the stack '%s'", ingressName, old.GetLabels()[model.StackNameLabel])
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

	selector := map[string]string{model.StackNameLabel: s.Name}
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
				return fmt.Errorf("Service '%s' has failed. Please check for errors and try again", podList[i].Labels[model.StackServiceNameLabel])
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

func addHiddenExposedPortsToStack(ctx context.Context, s *model.Stack) {
	for _, svc := range s.Services {
		addHiddenExposedPortsToSvc(ctx, svc, s.Namespace)
	}

}

func addHiddenExposedPortsToSvc(ctx context.Context, svc *model.Service, namespace string) {
	if svc.Image != "" {
		exposedPorts := registry.GetHiddenExposePorts(ctx, namespace, svc.Image)
		for _, port := range exposedPorts {
			if !model.IsAlreadyAdded(port, svc.Ports) {
				svc.Ports = append(svc.Ports, port)
			}
		}
	}
}
