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
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	forwardK8s "github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/k8s/jobs"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// DeployOptions represents the different options available for stack commands
type DeployOptions struct {
	Name             string
	Namespace        string
	Progress         string
	StackPaths       []string
	ServicesToDeploy []string
	Timeout          time.Duration
	ForceBuild       bool
	Wait             bool
	NoCache          bool
	InsidePipeline   bool
}

type buildTrackerInterface interface {
	TrackImageBuild(context.Context, *analytics.ImageBuildMetadata)
}

// Divert is the interface for the divert operations needed for stacks command
type Divert interface {
	UpdatePod(spec apiv1.PodSpec) apiv1.PodSpec
}

// Stack is the executor of stack commands
type Stack struct {
	K8sClient        kubernetes.Interface
	Config           *rest.Config
	AnalyticsTracker buildTrackerInterface
	Insights         buildTrackerInterface
	IoCtrl           *io.Controller
	Divert           Divert
	EndpointDeployer EndpointDeployer
}

const (
	maxRestartsToConsiderFailed = 3
)

// EndpointDeployer is an interface for deploying endpoints (Ingress or HTTPRoute)
type EndpointDeployer interface {
	// DeployServiceEndpoint deploys an endpoint for a specific service port
	DeployServiceEndpoint(ctx context.Context, name, serviceName string, port model.Port, stack *model.Stack) error
	// DeployComposeEndpoint deploys an endpoint from the compose endpoints spec
	DeployComposeEndpoint(ctx context.Context, name string, endpoint model.Endpoint, stack *model.Stack) error
}

// ShouldUseHTTPRoute determines if Gateway API HTTPRoute should be used instead of Ingress
// Returns true if gateway should be used, cluster metadata, and an error if configuration is invalid
// Priority order:
// 1. OKTETO_COMPOSE_ENDPOINTS_TYPE (feature flag) - if set, takes absolute precedence
// 2. OKTETO_DEFAULT_GATEWAY_TYPE (default) - only evaluated if feature flag is not set
func ShouldUseHTTPRoute() (bool, types.ClusterMetadata, error) {
	// Get gateway metadata from context
	octxGateway := okteto.GetContext().Gateway
	metadata := types.ClusterMetadata{}

	if octxGateway != nil {
		metadata.GatewayName = octxGateway.Name
		metadata.GatewayNamespace = octxGateway.Namespace
	}

	// Check feature flag (OKTETO_COMPOSE_ENDPOINTS_TYPE) - takes absolute precedence
	endpointType := os.Getenv(oktetoComposeEndpointsTypeEnvVar)
	if endpointType == "ingress" {
		oktetoLog.Infof("Using Ingress for endpoints (forced by %s=ingress)", oktetoComposeEndpointsTypeEnvVar)
		return false, types.ClusterMetadata{}, nil
	}
	if endpointType == "gateway" {
		oktetoLog.Infof("Using HTTPRoute for endpoints with the configured gateway %s/%s (forced by %s=gateway)", metadata.GatewayNamespace, metadata.GatewayName, oktetoComposeEndpointsTypeEnvVar)
		return true, metadata, nil
	}

	// Check default gateway type (OKTETO_DEFAULT_GATEWAY_TYPE)
	defaultGatewayType := os.Getenv(oktetoDefaultGatewayTypeEnvVar)
	if defaultGatewayType == "ingress" {
		oktetoLog.Infof("Using Ingress for endpoints (set by %s=ingress)", oktetoDefaultGatewayTypeEnvVar)
		return false, types.ClusterMetadata{}, nil
	}
	if defaultGatewayType == "gateway" {
		// If gateway is requested but metadata is empty, return error
		if metadata.GatewayName == "" || metadata.GatewayNamespace == "" {
			return false, types.ClusterMetadata{}, fmt.Errorf("gateway type requested via %s=gateway but gateway is not configured in the cluster", oktetoDefaultGatewayTypeEnvVar)
		}
		oktetoLog.Infof("Using HTTPRoute for endpoints with the configured gateway %s/%s (set by %s=gateway)", metadata.GatewayNamespace, metadata.GatewayName, oktetoDefaultGatewayTypeEnvVar)
		return true, metadata, nil
	}

	return true, metadata, nil
}

func (sd *Stack) RunDeploy(ctx context.Context, s *model.Stack, options *DeployOptions) error {

	analytics.TrackStackWarnings(s.Warnings.NotSupportedFields)

	if len(options.ServicesToDeploy) == 0 {
		definedServices := []string{}
		for serviceName := range s.Services {
			definedServices = append(definedServices, serviceName)
		}
		options.ServicesToDeploy = definedServices
	}

	err := sd.deployCompose(ctx, s, options)

	analytics.TrackDeployStack(err == nil, s.IsCompose)
	if err != nil {
		return err
	}
	oktetoLog.Success("Compose '%s' successfully deployed", s.Name)
	return nil
}

// Deploy deploys a stack
func (sd *Stack) deployCompose(ctx context.Context, s *model.Stack, options *DeployOptions) error {

	if err := validateServicesToDeploy(ctx, s, options, sd.K8sClient); err != nil {
		return err
	}

	if !options.InsidePipeline {
		if err := buildStackImages(ctx, s, options, sd.AnalyticsTracker, sd.Insights, sd.IoCtrl); err != nil {
			return err
		}
	}

	cfg := translateConfigMap(s)
	output := fmt.Sprintf("Deploying compose '%s'...", s.Name)
	cfg.Data[statusField] = progressingStatus
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	if err := configmaps.Deploy(ctx, cfg, s.Namespace, sd.K8sClient); err != nil {
		return err
	}

	err := deploy(ctx, s, sd.K8sClient, sd.Config, options, sd.Divert, sd.EndpointDeployer)
	if err != nil {
		output = fmt.Sprintf("%s\nCompose '%s' deployment failed: %s", output, s.Name, err.Error())
		cfg.Data[statusField] = errorStatus
		cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	} else {
		output = fmt.Sprintf("%s\nCompose '%s' successfully deployed", output, s.Name)
		cfg.Data[statusField] = deployedStatus
		cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	}

	if err := configmaps.Deploy(ctx, cfg, s.Namespace, sd.K8sClient); err != nil {
		return err
	}

	return err
}

// deploy deploys a stack to kubernetes
func deploy(ctx context.Context, s *model.Stack, c kubernetes.Interface, config *rest.Config, options *DeployOptions, divert Divert, endpointDeployer EndpointDeployer) error {
	DisplayWarnings(s)

	oktetoLog.Spinner(fmt.Sprintf("Deploying compose '%s'...", s.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {

		addImageMetadataToStack(s, options)

		// Determine deployer type for cleanup logic
		useHTTPRoute, _, err := ShouldUseHTTPRoute()
		if err != nil {
			exit <- err
			return
		}

		for _, serviceName := range options.ServicesToDeploy {
			if len(s.Services[serviceName].Ports) == 0 {
				continue
			}

			if err := deployK8sService(ctx, serviceName, s, c); err != nil {
				exit <- err
				return
			}
			// get the public ports from the compose service - this will be deployed into ingresses/httproutes
			ingressPortsToDeploy := getSvcPublicPorts(serviceName, s)
			for _, ingressPort := range ingressPortsToDeploy {
				ingressName := serviceName
				// If more than one port, ingressName will have <serviceName>-<PORT>, each port will have an ingress/httproute
				if len(ingressPortsToDeploy) > 1 {
					ingressName = fmt.Sprintf("%s-%d", serviceName, ingressPort.ContainerPort)
				}

				if err := endpointDeployer.DeployServiceEndpoint(ctx, ingressName, serviceName, ingressPort, s); err != nil {
					exit <- err
					return
				}
			}
		}

		servicesToDeploySet := map[string]bool{}
		for _, service := range options.ServicesToDeploy {
			servicesToDeploySet[service] = true
		}

		for _, name := range getVolumesToDeployFromServicesToDeploy(s, servicesToDeploySet) {
			if err := deployVolume(ctx, name, s, c); err != nil {
				exit <- err
				return
			}
		}

		if err := deployServices(ctx, s, c, config, options, divert); err != nil {
			exit <- err
			return
		}

		// compose has capacity to deploy endpoints for its services
		// each endpoint gets an ingress/httproute when using the endpoints spec at compose
		// the endpoint would have paths for services as defined at the spec
		for _, endpointName := range getEndpointsToDeployFromServicesToDeploy(s.Endpoints, servicesToDeploySet) {
			endpoint := s.Endpoints[endpointName]
			// initialize the maps for Labels and Annotations if nil
			if endpoint.Labels == nil {
				endpoint.Labels = map[string]string{}
			}
			if endpoint.Annotations == nil {
				endpoint.Annotations = map[string]string{}
			}

			// add specific stack labels
			if _, ok := endpoint.Labels[model.StackNameLabel]; !ok {
				endpoint.Labels[model.StackNameLabel] = format.ResourceK8sMetaString(s.Name)
			}
			if _, ok := endpoint.Labels[model.StackEndpointNameLabel]; !ok {
				endpoint.Labels[model.StackEndpointNameLabel] = endpointName
			}

			if err := endpointDeployer.DeployComposeEndpoint(ctx, endpointName, endpoint, s); err != nil {
				exit <- err
				return
			}
		}

		if err := destroyServicesNotInStack(ctx, s, c, config, useHTTPRoute); err != nil {
			exit <- err
			return
		}

		if !options.Wait {
			exit <- nil
			return
		}

		oktetoLog.Spinner("Waiting for services to be ready...")
		exit <- waitForPodsToBeRunning(ctx, s, options.ServicesToDeploy, c)
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		oktetoLog.StopSpinner()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func getVolumesToDeployFromServicesToDeploy(stack *model.Stack, servicesToDeploy map[string]bool) []string {

	volumesToDeploySet := map[string]bool{}

	for serviceName, serviceSpec := range stack.Services {
		if servicesToDeploy[serviceName] {
			for _, volume := range serviceSpec.Volumes {
				if stack.Volumes[volume.LocalPath] != nil {
					volumesToDeploySet[volume.LocalPath] = true
				}
			}
		}
	}

	volumesToDeploy := []string{}
	for name := range volumesToDeploySet {
		volumesToDeploy = append(volumesToDeploy, name)
	}

	return volumesToDeploy
}

func getEndpointsToDeployFromServicesToDeploy(endpoints model.EndpointSpec, servicesToDeploy map[string]bool) []string {
	endpointsToDeploySet := map[string]bool{}
	for name, spec := range endpoints {
		for _, rule := range spec.Rules {
			if servicesToDeploy[rule.Service] {
				endpointsToDeploySet[name] = true
			}
		}
	}

	endpointsToDeploy := []string{}
	for name := range endpointsToDeploySet {
		endpointsToDeploy = append(endpointsToDeploy, name)
	}

	return endpointsToDeploy
}

func deployServices(ctx context.Context, stack *model.Stack, k8sClient kubernetes.Interface, config *rest.Config, options *DeployOptions, divert Divert) error {
	deployedSvcs := make(map[string]bool)
	t := time.NewTicker(1 * time.Second)
	to := time.NewTicker(options.Timeout)

	// show an informational warning per service once
	serviceWarnings := make(map[string]string)
	restartsPerSvc := map[string]int{}
	for {
		select {
		case <-to.C:
			return fmt.Errorf("compose '%s' didn't finish after %s", stack.Name, options.Timeout.String())
		case <-t.C:
			for len(deployedSvcs) != len(options.ServicesToDeploy) {
				for _, svcName := range options.ServicesToDeploy {
					areAllDependenciesDeployed := true
					for dependentSvc := range stack.Services[svcName].DependsOn {
						if !deployedSvcs[dependentSvc] {
							areAllDependenciesDeployed = false
							break
						}
					}
					if deployedSvcs[svcName] || !areAllDependenciesDeployed {
						continue
					}

					if !canSvcBeDeployed(ctx, stack, svcName, k8sClient, config) {
						if failedJobs := getDependingFailedJobs(ctx, stack, svcName, k8sClient); len(failedJobs) > 0 {
							if len(failedJobs) == 1 {
								return fmt.Errorf("service '%s' dependency '%s' failed", svcName, failedJobs[0])
							}
							return fmt.Errorf("service '%s' dependencies '%s' failed", svcName, strings.Join(failedJobs, ", "))
						}
						if failedServices := getServicesWithFailedProbes(ctx, stack, svcName, k8sClient); len(failedServices) > 0 {
							for service, err := range failedServices {
								errMessage := fmt.Errorf("service '%s' cannot be deployed because dependent service '%s' is failing its healthcheck probes: %s", svcName, service, err)
								if errors.Is(err, oktetoErrors.ErrLivenessProbeFailed) {
									return errMessage
								} else if errors.Is(err, oktetoErrors.ErrReadinessProbeFailed) {
									if _, ok := serviceWarnings[service]; !ok {
										oktetoLog.Information(errMessage.Error())
										serviceWarnings[service] = errMessage.Error()
									}
								}
							}
						}
						if err := getErrorDueToRestartLimit(ctx, stack, svcName, restartsPerSvc, k8sClient); err != nil {
							return err
						}
						continue
					}
					oktetoLog.Spinner(fmt.Sprintf("Deploying service '%s'...", svcName))
					err := deploySvc(ctx, stack, svcName, k8sClient, divert)
					if err != nil {
						return err
					}
					deployedSvcs[svcName] = true
					oktetoLog.Spinner("Waiting for services to be ready...")
				}
			}
			return nil
		}
	}
}

func deploySvc(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, divert Divert) error {
	isNew := false
	var err error
	if stack.Services[svcName].IsJob() {
		isNew, err = deployJob(ctx, svcName, stack, client, divert)
	} else if len(stack.Services[svcName].Volumes) == 0 {
		isNew, err = deployDeployment(ctx, svcName, stack, client, divert)
	} else {
		isNew, err = deployStatefulSet(ctx, svcName, stack, client, divert)
	}

	if err != nil {
		if strings.Contains(err.Error(), "skipping ") {
			oktetoLog.Warning(err.Error())
			return nil
		}
		return err
	}
	if isNew {
		oktetoLog.Success("Service '%s' created", svcName)
	} else {
		oktetoLog.Success("Service '%s' updated", svcName)
	}

	return nil
}

func canSvcBeDeployed(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface, config *rest.Config) bool {
	for dependentSvc, condition := range stack.Services[svcName].DependsOn {
		if !isSvcReady(ctx, stack, dependentSvc, condition, client, config) {
			oktetoLog.Infof("Service %s can not be deployed due to %s", svcName, dependentSvc)
			return false
		}
	}
	return true
}

func getServicesWithFailedProbes(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface) map[string]error {
	svc := stack.Services[svcName]
	dependingServicesHealthcheckMap := make(map[string]*model.HealthCheck)
	for dependingSvc, condition := range svc.DependsOn {
		healthcheck := stack.Services[dependingSvc].Healtcheck
		if healthcheck != nil && condition.Condition == model.DependsOnServiceHealthy {
			dependingServicesHealthcheckMap[dependingSvc] = stack.Services[dependingSvc].Healtcheck
		}
	}
	failedServices := make(map[string]error)
	for svcName, healthcheck := range dependingServicesHealthcheckMap {
		if err := pods.GetHealthcheckFailure(ctx, client, stack.Namespace, svcName, stack.Name, healthcheck); err != nil {
			failedServices[svcName] = err
		}
	}
	return failedServices
}

func getErrorDueToRestartLimit(ctx context.Context, stack *model.Stack, svcName string, restartsPerSvc map[string]int, client kubernetes.Interface) error {
	svc := stack.Services[svcName]
	for dependingSvc := range svc.DependsOn {
		svcLabels := map[string]string{model.StackNameLabel: format.ResourceK8sMetaString(stack.Name), model.StackServiceNameLabel: dependingSvc}
		p, err := pods.GetBySelector(ctx, stack.Namespace, svcLabels, client)
		if err != nil {
			oktetoLog.Infof("could not get pod of svc '%s': %s", dependingSvc, err)
			continue
		}
		if _, ok := restartsPerSvc[dependingSvc]; !ok {
			restartsPerSvc[dependingSvc] = getPodRestarts(p)
		}

		totalRestarts := getPodRestarts(p)
		restartsFromThisDeploy := totalRestarts - restartsPerSvc[dependingSvc]
		maxSvcRestarts := stack.Services[dependingSvc].BackOffLimit
		if maxSvcRestarts == 0 {
			maxSvcRestarts = maxRestartsToConsiderFailed
		}

		if int32(restartsFromThisDeploy) >= maxSvcRestarts {
			return fmt.Errorf("service '%s' has been restarted %d times within this deploy. Please check the logs and try again", dependingSvc, restartsFromThisDeploy)
		}
	}

	return nil
}

func getPodRestarts(p *apiv1.Pod) int {
	totalRestarts := 0
	for _, cStatus := range p.Status.ContainerStatuses {
		totalRestarts += int(cStatus.RestartCount)
	}
	return totalRestarts
}

func getDependingFailedJobs(ctx context.Context, stack *model.Stack, svcName string, client kubernetes.Interface) []string {
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
	svcLabels := map[string]string{model.StackNameLabel: format.ResourceK8sMetaString(stack.Name), model.StackServiceNameLabel: svcName}

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
	forwarder := forwardK8s.NewPortForwardManager(ctx, model.Localhost, config, client, stack.Namespace)
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
		if err := forwarder.Add(forward.Forward{Local: port, Remote: int(p.ContainerPort)}); err != nil {
			continue
		}
	}
	if err := forwarder.Start(podName, stack.Namespace); err != nil {
		oktetoLog.Infof("could not start port-forward: %s", err)
	}
	defer forwarder.Stop()
	for _, port := range portsToTest {
		p := strconv.Itoa(port)
		url := net.JoinHostPort(model.Localhost, p)
		_, err := net.Dial("tcp", url)
		if err != nil {
			continue
		}
		return true
	}
	return false
}

func deployK8sService(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface) error {
	svcK8s := translateService(svcName, s)
	old, err := services.Get(ctx, svcName, s.Namespace, c)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return fmt.Errorf("error getting service '%s': %w", svcName, err)
		}
		if err := services.Deploy(ctx, svcK8s, c); err != nil {
			return err
		}
		oktetoLog.Success("Kubernetes service '%s' created", svcName)
		return nil
	}

	if old.GetLabels()[model.StackNameLabel] == "" {
		oktetoLog.Warning("skipping deploy of kubernetes service %s due to name collision: the service '%s' was running before deploying your compose", svcName, svcName)
		return nil
	}

	if old.GetLabels()[model.StackNameLabel] != svcK8s.GetLabels()[model.StackNameLabel] {
		oktetoLog.Warning("skipping creation of kubernetes '%s' due to name collision with endpoint in compose '%s'", svcName, old.GetLabels()[model.StackNameLabel])
		return nil
	}

	svcK8s.ObjectMeta.ResourceVersion = old.ObjectMeta.ResourceVersion
	if err := services.Deploy(ctx, svcK8s, c); err != nil {
		return err
	}
	oktetoLog.Success("Kubernetes service '%s' updated", svcName)
	return nil
}

func deployDeployment(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface, divert Divert) (bool, error) {
	d := translateDeployment(svcName, s, divert)
	old, err := c.AppsV1().Deployments(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return false, fmt.Errorf("error getting deployment of service '%s': %w", svcName, err)
	}
	isNewDeployment := old == nil || old.Name == ""
	if !isNewDeployment {
		if old.Labels[model.StackNameLabel] == "" {
			return false, fmt.Errorf("skipping deploy of deployment '%s' due to name collision with pre-existing deployment", svcName)
		}
		// PR 2742 https://github.com/okteto/okteto/pull/2742
		// we are introducing this check for the old stack label as we resolved the bug
		// when the stack is under an .okteto folder, this was the name for the dev environment
		// for those users which will have a dev environment deployed with old version
		// when re-deploying we switch the name for the environment and we have to move the resources to the new name
		if old.Labels[model.StackNameLabel] != format.ResourceK8sMetaString(s.Name) && old.Labels[model.StackNameLabel] != "okteto" {
			return false, fmt.Errorf("skipping deploy of deployment '%s' due to name collision with deployment in compose '%s'", svcName, old.Labels[model.StackNameLabel])
		}
		if v, ok := old.Labels[model.DeployedByLabel]; ok {
			d.Labels[model.DeployedByLabel] = v
			if old.Labels[model.StackNameLabel] == "okteto" {
				d.Labels[model.DeployedByLabel] = format.ResourceK8sMetaString(s.Name)
			}
		}
	}

	if !isNewDeployment && old.Labels[model.StackNameLabel] == "okteto" {
		if err := deployments.Destroy(ctx, old.Name, old.Namespace, c); err != nil {
			return false, fmt.Errorf("error updating deployment of service '%s': %w", svcName, err)
		}
		if _, err := deployments.Deploy(ctx, d, c); err != nil {
			return false, fmt.Errorf("error updating deployment of service '%s': %w", svcName, err)
		}
		return isNewDeployment, nil
	}

	if _, err := deployments.Deploy(ctx, d, c); err != nil {
		if isNewDeployment {
			return false, fmt.Errorf("error creating deployment of service '%s': %w", svcName, err)
		}
		return false, fmt.Errorf("error updating deployment of service '%s': %w", svcName, err)
	}

	return isNewDeployment, nil
}

func deployStatefulSet(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface, divert Divert) (bool, error) {
	sfs := translateStatefulSet(svcName, s, divert)
	old, err := c.AppsV1().StatefulSets(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return false, fmt.Errorf("error getting statefulset of service '%s': %w", svcName, err)
	}
	if old == nil || old.Name == "" {
		if _, err := statefulsets.Deploy(ctx, sfs, c); err != nil {
			return false, fmt.Errorf("error creating statefulset of service '%s': %w", svcName, err)
		}
		return true, nil
	}

	if old.Labels[model.StackNameLabel] == "" {
		return false, fmt.Errorf("skipping deploy of statefulset '%s' due to name collision with pre-existing statefulset", svcName)
	}
	if old.Labels[model.StackNameLabel] != format.ResourceK8sMetaString(s.Name) && old.Labels[model.StackNameLabel] != "okteto" {
		return false, fmt.Errorf("skipping deploy of statefulset '%s' due to name collision with statefulset in compose '%s'", svcName, old.Labels[model.StackNameLabel])
	}
	if v, ok := old.Labels[model.DeployedByLabel]; ok {
		sfs.Labels[model.DeployedByLabel] = v
		if old.Labels[model.StackNameLabel] == "okteto" {
			sfs.Labels[model.DeployedByLabel] = format.ResourceK8sMetaString(s.Name)
		}
	}
	if _, err := statefulsets.Deploy(ctx, sfs, c); err != nil {
		if !strings.Contains(err.Error(), "Forbidden: updates to statefulset spec") {
			return false, fmt.Errorf("error updating statefulset of service '%s': %w", svcName, err)
		}
		if err := statefulsets.Destroy(ctx, sfs.Name, sfs.Namespace, c); err != nil {
			return false, fmt.Errorf("error updating statefulset of service '%s': %w", svcName, err)
		}
		if _, err := statefulsets.Deploy(ctx, sfs, c); err != nil {
			return false, fmt.Errorf("error updating statefulset of service '%s': %w", svcName, err)
		}
	}

	return false, nil
}

func deployJob(ctx context.Context, svcName string, s *model.Stack, c kubernetes.Interface, divert Divert) (bool, error) {
	job := translateJob(svcName, s, divert)
	old, err := c.BatchV1().Jobs(s.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return false, fmt.Errorf("error getting job of service '%s': %w", svcName, err)
	}
	isNewJob := old == nil || old.Name == ""
	if !isNewJob {
		if old.Labels[model.StackNameLabel] == "" {
			return false, fmt.Errorf("skipping deploy of job '%s' due to name collision with pre-existing job", svcName)
		}
		if old.Labels[model.StackNameLabel] != format.ResourceK8sMetaString(s.Name) && old.Labels[model.StackNameLabel] != "okteto" {
			return false, fmt.Errorf("skipping deploy of job '%s' due to name collision with job in stack '%s'", svcName, old.Labels[model.StackNameLabel])
		}
	}

	if isNewJob {
		if err := jobs.Create(ctx, job, c); err != nil {
			return false, fmt.Errorf("error creating job of service '%s': %w", svcName, err)
		}
	} else {
		if err := jobs.Update(ctx, job, c); err != nil {
			return false, fmt.Errorf("error updating job of service '%s': %w", svcName, err)
		}
	}
	return isNewJob, nil
}

func deployVolume(ctx context.Context, volumeName string, s *model.Stack, c kubernetes.Interface) error {
	pvc := translatePersistentVolumeClaim(volumeName, s)

	old, err := c.CoreV1().PersistentVolumeClaims(s.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error getting volume '%s': %w", pvc.Name, err)
	}
	if old == nil || old.Name == "" {
		if err := volumes.Create(ctx, &pvc, c); err != nil {
			return fmt.Errorf("error creating volume '%s': %w", pvc.Name, err)
		}
		oktetoLog.Success("Volume '%s' created", volumeName)
	} else {
		if old.Labels[model.StackNameLabel] == "" {
			oktetoLog.Warning("skipping creation of volume '%s' due to name collision with pre-existing volume", pvc.Name)
			return nil
		}
		if old.Labels[model.StackNameLabel] != format.ResourceK8sMetaString(s.Name) && old.Labels[model.StackNameLabel] != "okteto" {
			oktetoLog.Warning("skipping creation of volume '%s' due to name collision with volume in stack '%s'", pvc.Name, old.Labels[model.StackNameLabel])
			return nil
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
			return fmt.Errorf("error updating volume '%s': %w", old.Name, err)
		}
		oktetoLog.Success("Volume '%s' updated", volumeName)
	}
	return nil
}

func waitForPodsToBeRunning(ctx context.Context, s *model.Stack, servicesToDeploy []string, c kubernetes.Interface) error {
	var numPods int32 = 0
	cacheServicesToDeploy := map[string]bool{}
	for _, svc := range servicesToDeploy {
		cacheServicesToDeploy[svc] = true
	}

	for name, svc := range s.Services {
		// If there is a subset of services to deploy, and the service is not there, skip it
		// Otherwise (the services is among the subset to deploy, or there is no subset and everything was deployed),
		// count it
		if len(cacheServicesToDeploy) > 0 && !cacheServicesToDeploy[name] {
			continue
		}
		numPods += svc.Replicas
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	timeoutDuration := 600 * time.Second
	timeout := time.Now().Add(timeoutDuration)

	selector := map[string]string{model.StackNameLabel: format.ResourceK8sMetaString(s.Name)}
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
				return fmt.Errorf("service '%s' has failed. Please check for errors and try again", podList[i].Labels[model.StackServiceNameLabel])
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
			oktetoLog.Warning("'%s' field is not currently supported and will be ignored.", warnings[0])
		} else {
			notSupportedFields := strings.Join(model.GroupWarningsBySvc(warnings), "\n  - ")
			oktetoLog.Warning("The following fields are not currently supported and will be ignored: \n  - %s", notSupportedFields)
		}
		oktetoLog.Yellow("Help us to decide which fields to implement next by filing an issue in https://github.com/okteto/okteto/issues/new")
	}
}

func DisplayVolumeMountWarnings(warnings []string) {
	for _, warning := range warnings {
		oktetoLog.Warning(warning)
	}
}

func DisplaySanitizedServicesWarnings(previousToNewNameMap map[string]string) {
	for previousName, newName := range previousToNewNameMap {
		oktetoLog.Warning("Service '%s' specified in compose file has been sanitized into '%s'. This may affect discovery service.", previousName, newName)
	}
}

func addImageMetadataToStack(s *model.Stack, options *DeployOptions) {
	for _, svcName := range options.ServicesToDeploy {
		svc := s.Services[svcName]
		addImageMetadataToSvc(svc)
	}

}

func addImageMetadataToSvc(svc *model.Service) {
	if svc.Image != "" {
		reg := registry.NewOktetoRegistry(okteto.Config{})
		imageMetadata, err := reg.GetImageMetadata(svc.Image)
		if err != nil {
			oktetoLog.Infof("could not add image metadata: %s", err)
			return
		}

		if reg.IsOktetoRegistry(svc.Image) {
			svc.Image = imageMetadata.Image
		}
		for _, port := range imageMetadata.Ports {
			if !model.IsAlreadyAdded(port, svc.Ports) {
				svc.Ports = append(svc.Ports, model.Port{ContainerPort: port.ContainerPort, Protocol: port.Protocol})
			}
		}
	}
}

func validateServicesToDeploy(ctx context.Context, s *model.Stack, options *DeployOptions, c kubernetes.Interface) error {
	if err := ValidateDefinedServices(s, options.ServicesToDeploy); err != nil {
		return err
	}
	if !options.InsidePipeline {
		options.ServicesToDeploy = AddDependentServicesIfNotPresent(ctx, s, options.ServicesToDeploy, c)
	}
	return nil
}

// ValidateDefinedServices checks that the services to deploy are in the compose file
func ValidateDefinedServices(s *model.Stack, servicesToDeploy []string) error {
	for _, svcToDeploy := range servicesToDeploy {
		if _, ok := s.Services[svcToDeploy]; !ok {
			definedSvcs := make([]string, 0)
			for svcName := range s.Services {
				definedSvcs = append(definedSvcs, svcName)
			}
			return fmt.Errorf("service '%s' is not defined. Defined services are: [%s]", svcToDeploy, strings.Join(definedSvcs, ", "))
		}
	}
	if err := s.Services.ValidateDependsOn(servicesToDeploy); err != nil {
		return err
	}
	return nil
}

// AddDependentServicesIfNotPresent adds dependands services to deploy
func AddDependentServicesIfNotPresent(ctx context.Context, s *model.Stack, svcsToDeploy []string, c kubernetes.Interface) []string {
	initialSvcsToDeploy := svcsToDeploy
	svcsToDeployWithDependencies := addDependentServices(ctx, s, svcsToDeploy, c)
	if len(initialSvcsToDeploy) != len(svcsToDeploy) {
		added := getAddedSvcs(initialSvcsToDeploy, svcsToDeployWithDependencies)

		oktetoLog.Warning("The following services need to be deployed because the services passed as arguments depend on them: [%s]", strings.Join(added, ", "))
	}
	return svcsToDeployWithDependencies
}

func addDependentServices(ctx context.Context, s *model.Stack, svcsToDeploy []string, c kubernetes.Interface) []string {
	initialLength := len(svcsToDeploy)
	svcsToDeploySet := map[string]bool{}
	for _, svc := range svcsToDeploy {
		svcsToDeploySet[svc] = true
	}
	for _, svcToDeploy := range svcsToDeploy {
		for dependentSvc := range s.Services[svcToDeploy].DependsOn {
			if _, ok := svcsToDeploySet[dependentSvc]; ok {
				continue
			}
			if !isSvcRunning(ctx, s.Services[dependentSvc], s.Namespace, dependentSvc, c) {
				svcsToDeploy = append(svcsToDeploy, dependentSvc)
				svcsToDeploySet[dependentSvc] = true
			}
		}
	}
	if initialLength != len(svcsToDeploy) {
		return addDependentServices(ctx, s, svcsToDeploy, c)
	}
	return svcsToDeploy
}

func getAddedSvcs(initialSvcsToDeploy, svcsToDeployWithDependencies []string) []string {
	initialSvcsToDeploySet := map[string]bool{}
	for _, svc := range initialSvcsToDeploy {
		initialSvcsToDeploySet[svc] = true
	}
	added := []string{}
	for _, svcName := range svcsToDeployWithDependencies {
		if _, ok := initialSvcsToDeploySet[svcName]; ok {
			added = append(added, svcName)
		}
	}
	return added
}
