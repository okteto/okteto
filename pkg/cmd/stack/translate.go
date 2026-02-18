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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	NameField    = "name"
	statusField  = "status"
	YamlField    = "yaml"
	ComposeField = "compose"
	outputField  = "output"

	progressingStatus = "progressing"
	deployedStatus    = "deployed"
	errorStatus       = "error"
	destroyingStatus  = "destroying"

	pvcName = "pvc"

	// oktetoComposeVolumeAffinityEnabledEnvVar represents whether the feature flag to enable volume affinity is enabled or not
	oktetoComposeVolumeAffinityEnabledEnvVar = "OKTETO_COMPOSE_VOLUME_AFFINITY_ENABLED"

	// oktetoComposeEndpointsTypeEnvVar defines the endpoint type: "gateway" or "ingress". Defaults to automatic detection based on cluster gateway metadata.
	oktetoComposeEndpointsTypeEnvVar = "OKTETO_COMPOSE_ENDPOINTS_TYPE"

	// oktetoDefaultGatewayTypeEnvVar defines the default gateway type: "gateway" or "ingress". Only used if oktetoComposeEndpointsTypeEnvVar is not set.
	oktetoDefaultGatewayTypeEnvVar = "OKTETO_DEFAULT_GATEWAY_TYPE"

	// dependsOnAnnotation represents the annotation to define the depends_on field
	dependsOnAnnotation = "dev.okteto.com/depends-on"
)

// +enum
type updateStrategy string

const (
	// rollingUpdateStrategy represent a rolling update strategy
	rollingUpdateStrategy updateStrategy = "rolling"

	// recreateUpdateStrategy represents a recreate update strategy
	recreateUpdateStrategy updateStrategy = "recreate"

	// onDeleteUpdateStrategy represents a recreate update strategy
	onDeleteUpdateStrategy updateStrategy = "on-delete"
)

func buildStackImages(ctx context.Context, s *model.Stack, options *DeployOptions, analyticsTracker, insights buildTrackerInterface, ioCtrl *io.Controller) error {
	manifest := model.NewManifestFromStack(s)

	onBuildFinish := []buildv2.OnBuildFinish{
		analyticsTracker.TrackImageBuild,
		insights.TrackImageBuild,
	}
	okCtx := &okteto.ContextStateless{
		Store: okteto.GetContextStore(),
	}
	builder := buildv2.NewBuilderFromScratch(ioCtrl, onBuildFinish, buildCmd.GetBuildkitConnector(okCtx, ioCtrl))
	if options.ForceBuild {
		buildOptions := &types.BuildOptions{
			Manifest:    manifest,
			CommandArgs: options.ServicesToDeploy,
		}
		if err := builder.Build(ctx, buildOptions); err != nil {
			return err
		}
	} else {
		svcsToBuild, err := builder.GetServicesToBuildDuringExecution(ctx, manifest, options.ServicesToDeploy)
		if err != nil {
			return err
		}

		if len(svcsToBuild) != 0 {
			buildOptions := &types.BuildOptions{
				CommandArgs: svcsToBuild,
				Manifest:    manifest,
			}
			if err := builder.Build(ctx, buildOptions); err != nil {
				return err
			}
		}
	}
	*s = *manifest.Deploy.ComposeSection.Stack
	return nil
}

func translateConfigMap(s *model.Stack) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.GetStackConfigMapName(s.Name),
			Labels: map[string]string{
				model.StackLabel:      "true",
				model.DeployedByLabel: format.ResourceK8sMetaString(s.Name),
			},
		},
		Data: map[string]string{
			NameField:    s.Name,
			YamlField:    base64.StdEncoding.EncodeToString(s.Manifest),
			ComposeField: strconv.FormatBool(s.IsCompose),
		},
	}
}

func translateDeployment(svcName string, s *model.Stack, divert Divert) *appsv1.Deployment {
	svc := s.Services[svcName]

	svcHealthchecks := getSvcHealthProbe(svc)

	podSpec := apiv1.PodSpec{
		TerminationGracePeriodSeconds: ptr.To(svc.StopGracePeriod),
		NodeSelector:                  svc.NodeSelector,
		Containers: []apiv1.Container{
			{
				Name:            svcName,
				Image:           svc.Image,
				Command:         svc.Entrypoint.Values,
				Args:            svc.Command.Values,
				Env:             translateServiceEnvironment(svc),
				Ports:           translateContainerPorts(svc),
				SecurityContext: translateSecurityContext(svc),
				Resources:       translateResources(svc),
				WorkingDir:      svc.Workdir,
				ReadinessProbe:  svcHealthchecks.readiness,
				LivenessProbe:   svcHealthchecks.liveness,
			},
		},
	}

	if divert != nil {
		podSpec = divert.UpdatePod(podSpec)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(svc.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: translateLabelSelector(svcName, s),
			},
			Strategy: getDeploymentUpdateStrategy(svc),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: podSpec,
			},
		},
	}
}

func translatePersistentVolumeClaim(volumeName string, s *model.Stack) apiv1.PersistentVolumeClaim {
	volumeSpec := s.Volumes[volumeName]
	labels := translateVolumeLabels(volumeName, s)
	pvc := apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        volumeName,
			Namespace:   s.Namespace,
			Labels:      labels,
			Annotations: volumeSpec.Annotations,
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
			Resources: apiv1.VolumeResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": volumeSpec.Size.Value,
				},
			},
			StorageClassName: translateStorageClass(volumeSpec.Class),
		},
	}
	return pvc
}

func translateStatefulSet(svcName string, s *model.Stack, divert Divert) *appsv1.StatefulSet {
	svc := s.Services[svcName]

	initContainers := getInitContainers(svcName, s)
	svcHealthchecks := getSvcHealthProbe(svc)

	podSpec := apiv1.PodSpec{
		TerminationGracePeriodSeconds: ptr.To(svc.StopGracePeriod),
		InitContainers:                initContainers,
		Affinity:                      translateAffinity(svc),
		NodeSelector:                  svc.NodeSelector,
		Volumes:                       translateVolumes(svc),
		Containers: []apiv1.Container{
			{
				Name:            svcName,
				Image:           svc.Image,
				Command:         svc.Entrypoint.Values,
				Args:            svc.Command.Values,
				Env:             translateServiceEnvironment(svc),
				Ports:           translateContainerPorts(svc),
				SecurityContext: translateSecurityContext(svc),
				VolumeMounts:    translateVolumeMounts(svc),
				Resources:       translateResources(svc),
				WorkingDir:      svc.Workdir,
				ReadinessProbe:  svcHealthchecks.readiness,
				LivenessProbe:   svcHealthchecks.liveness,
			},
		},
	}

	if divert != nil {
		podSpec = divert.UpdatePod(podSpec)
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             ptr.To(svc.Replicas),
			RevisionHistoryLimit: ptr.To(int32(2)),
			Selector: &metav1.LabelSelector{
				MatchLabels: translateLabelSelector(svcName, s),
			},
			UpdateStrategy: getStatefulsetUpdateStrategy(svc),
			ServiceName:    svcName,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: podSpec,
			},
			VolumeClaimTemplates: translateVolumeClaimTemplates(svcName, s),
		},
	}
}

func translateJob(svcName string, s *model.Stack, divert Divert) *batchv1.Job {
	svc := s.Services[svcName]

	initContainers := getInitContainers(svcName, s)
	svcHealthchecks := getSvcHealthProbe(svc)
	podSpec := apiv1.PodSpec{
		RestartPolicy:                 svc.RestartPolicy,
		TerminationGracePeriodSeconds: ptr.To(svc.StopGracePeriod),
		InitContainers:                initContainers,
		Affinity:                      translateAffinity(svc),
		NodeSelector:                  svc.NodeSelector,
		Containers: []apiv1.Container{
			{
				Name:            svcName,
				Image:           svc.Image,
				Command:         svc.Entrypoint.Values,
				Args:            svc.Command.Values,
				Env:             translateServiceEnvironment(svc),
				Ports:           translateContainerPorts(svc),
				SecurityContext: translateSecurityContext(svc),
				VolumeMounts:    translateVolumeMounts(svc),
				Resources:       translateResources(svc),
				WorkingDir:      svc.Workdir,
				ReadinessProbe:  svcHealthchecks.readiness,
				LivenessProbe:   svcHealthchecks.liveness,
			},
		},
		Volumes: translateVolumes(svc),
	}

	if divert != nil {
		podSpec = divert.UpdatePod(podSpec)
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: batchv1.JobSpec{
			Completions:  ptr.To(svc.Replicas),
			Parallelism:  ptr.To(int32(1)),
			BackoffLimit: &svc.BackOffLimit,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: podSpec,
			},
		},
	}
}

func getInitContainers(svcName string, s *model.Stack) []apiv1.Container {
	svc := s.Services[svcName]
	initContainers := []apiv1.Container{}
	if len(svc.Volumes) > 0 {
		addPermissionsContainer := getAddPermissionsInitContainer(svcName, svc)
		initContainers = append(initContainers, addPermissionsContainer)

	}
	initializationContainer := getInitializeVolumeContentContainer(svcName, svc)
	if initializationContainer != nil {
		initContainers = append(initContainers, *initializationContainer)
	}

	return initContainers
}

func getAddPermissionsInitContainer(svcName string, svc *model.Service) apiv1.Container {
	initContainerCommand, initContainerVolumeMounts := getInitContainerCommandAndVolumeMounts(*svc)
	initContainer := apiv1.Container{
		Name:            fmt.Sprintf("init-%s", svcName),
		Image:           config.NewImageConfig(oktetoLog.GetOutputWriter()).GetCliImage(),
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         initContainerCommand,
		VolumeMounts:    initContainerVolumeMounts,
	}
	return initContainer
}

func getInitializeVolumeContentContainer(svcName string, svc *model.Service) *apiv1.Container {
	c := &apiv1.Container{
		Name:            fmt.Sprintf("init-volume-%s", svcName),
		Image:           svc.Image,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		VolumeMounts:    []apiv1.VolumeMount{},
	}
	command := "echo initializing volume..."
	for idx, v := range svc.Volumes {
		subpath := fmt.Sprintf("data-%d", idx)
		if v.LocalPath != "" {
			subpath = v.LocalPath
		}
		c.VolumeMounts = append(
			c.VolumeMounts,
			apiv1.VolumeMount{
				Name:      getVolumeClaimName(&v),
				MountPath: fmt.Sprintf("/init-volume-%d", idx),
				SubPath:   subpath,
			},
		)
		command = fmt.Sprintf("%s && (cp -Rv %s/. /init-volume-%d || true)", command, v.RemotePath, idx)
	}
	if len(c.VolumeMounts) != 0 {
		c.Command = []string{"sh", "-c", command}
		return c
	}
	return nil
}

func getInitContainerCommandAndVolumeMounts(svc model.Service) ([]string, []apiv1.VolumeMount) {
	volumeMounts := make([]apiv1.VolumeMount, 0)

	var command string
	var addedVolumesVolume, addedDataVolume bool
	for _, volume := range svc.Volumes {
		volumeName := getVolumeClaimName(&volume)
		if volumeName != pvcName {
			volumeMounts = append(volumeMounts, apiv1.VolumeMount{Name: volumeName, MountPath: fmt.Sprintf("/volumes/%s", volumeName)})
			if !addedVolumesVolume {
				if command == "" {
					command = "chmod 777 /volumes/*"
					addedVolumesVolume = true
				} else {
					command += " && chmod 777 /volumes/*"
				}
			}
		} else if !addedDataVolume {
			volumeMounts = append(volumeMounts, apiv1.VolumeMount{Name: volumeName, MountPath: "/data"})
			if command == "" {
				command = "chmod 777 /data"
				addedDataVolume = true
			} else {
				command += " && chmod 777 /data"
			}
		}
	}
	return []string{"sh", "-c", command}, volumeMounts
}

func translateVolumeClaimTemplates(svcName string, s *model.Stack) []apiv1.PersistentVolumeClaim {
	svc := s.Services[svcName]
	for _, volume := range svc.Volumes {
		if volume.LocalPath == "" {
			return []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        pvcName,
						Labels:      translateLabels(svcName, s),
						Annotations: translateAnnotations(svc),
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
						Resources: apiv1.VolumeResourceRequirements{
							Requests: apiv1.ResourceList{
								"storage": svc.Resources.Requests.Storage.Size.Value,
							},
						},
						StorageClassName: translateStorageClass(svc.Resources.Requests.Storage.Class),
					},
				},
			}
		}
	}
	return nil
}

func translateVolumes(svc *model.Service) []apiv1.Volume {
	volumes := make([]apiv1.Volume, 0)
	for _, volume := range svc.Volumes {
		name := getVolumeClaimName(&volume)
		volumes = append(volumes, apiv1.Volume{
			Name: name,
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: volume.LocalPath,
				},
			},
		})
	}

	return volumes
}

func translateService(svcName string, s *model.Stack) *apiv1.Service {
	svc := s.Services[svcName]
	annotations := translateAnnotations(svc)

	serviceSpec := apiv1.ServiceSpec{
		Selector: translateLabelSelector(svcName, s),
		Type:     apiv1.ServiceTypeClusterIP,
		Ports:    translateServicePorts(*svc),
	}

	// Configure headless service for DNS round-robin endpoint mode
	if svc.EndpointMode == model.EndpointModeDNSRR {
		serviceSpec.ClusterIP = "None"
	}

	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: annotations,
		},
		Spec: serviceSpec,
	}
}

func getSvcPublicPorts(svcName string, s *model.Stack) []model.Port {
	result := []model.Port{}
	for _, p := range s.Services[svcName].Ports {
		if !model.IsSkippablePort(p.ContainerPort) && p.HostPort != 0 {
			result = append(result, p)
		}
	}
	return result
}

func translateVolumeLabels(volumeName string, s *model.Stack) map[string]string {
	volume := s.Volumes[volumeName]
	labels := map[string]string{
		model.StackNameLabel:       format.ResourceK8sMetaString(s.Name),
		model.StackVolumeNameLabel: volumeName,
		model.DeployedByLabel:      format.ResourceK8sMetaString(s.Name),
	}
	for k := range volume.Labels {
		labels[k] = volume.Labels[k]
	}
	return labels
}

func translateAffinity(svc *model.Service) *apiv1.Affinity {
	if !env.LoadBooleanOrDefault(oktetoComposeVolumeAffinityEnabledEnvVar, true) {
		return nil
	}

	requirements := make([]apiv1.PodAffinityTerm, 0)
	for _, volume := range svc.Volumes {
		if volume.LocalPath == "" {
			continue
		}
		requirements = append(requirements, apiv1.PodAffinityTerm{
			TopologyKey: "kubernetes.io/hostname",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      fmt.Sprintf("%s-%s", model.StackVolumeNameLabel, volume.LocalPath),
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
		},
		)
	}
	if len(requirements) > 0 {
		return &apiv1.Affinity{
			PodAffinity: &apiv1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: requirements,
			},
		}
	}

	return nil
}

func translateLabels(svcName string, s *model.Stack) map[string]string {
	svc := s.Services[svcName]
	labels := map[string]string{
		model.StackNameLabel:        format.ResourceK8sMetaString(s.Name),
		model.StackServiceNameLabel: svcName,
		model.DeployedByLabel:       format.ResourceK8sMetaString(s.Name),
	}
	for k := range svc.Labels {
		labels[k] = svc.Labels[k]
	}

	for _, volume := range svc.Volumes {
		if volume.LocalPath != "" {
			labels[fmt.Sprintf("%s-%s", model.StackVolumeNameLabel, volume.LocalPath)] = "true"
		}
	}
	return labels
}

func translateLabelSelector(svcName string, s *model.Stack) map[string]string {
	labels := map[string]string{
		model.StackNameLabel:        format.ResourceK8sMetaString(s.Name),
		model.StackServiceNameLabel: svcName,
	}
	return labels
}

func translateAnnotations(svc *model.Service) map[string]string {
	result := getAnnotations(svc)
	for k, v := range svc.Annotations {
		result[k] = v
	}
	return result
}

func getAnnotations(svc *model.Service) map[string]string {
	annotations := map[string]string{}
	if utils.IsOktetoRepo() {
		annotations[model.OktetoSampleAnnotation] = "true"
	}
	if len(svc.DependsOn) > 0 {
		dependsOn, err := json.Marshal(svc.DependsOn)
		if err != nil {
			oktetoLog.Infof("error marshalling depends_on annotation: %s", err)
		} else {
			annotations[dependsOnAnnotation] = string(dependsOn)
		}
	}

	return annotations
}

func translateVolumeMounts(svc *model.Service) []apiv1.VolumeMount {
	result := []apiv1.VolumeMount{}
	for i, v := range svc.Volumes {
		name := getVolumeClaimName(&v)
		subpath := fmt.Sprintf("data-%d", i)
		if v.LocalPath != "" {
			subpath = v.LocalPath
		}
		result = append(
			result,
			apiv1.VolumeMount{
				MountPath: v.RemotePath,
				Name:      name,
				SubPath:   subpath,
			},
		)
	}

	return result
}

func getVolumeClaimName(v *build.VolumeMounts) string {
	var name string
	if v.LocalPath != "" {
		name = v.LocalPath
	} else {
		name = pvcName
	}
	return name
}

func translateSecurityContext(svc *model.Service) *apiv1.SecurityContext {
	if len(svc.CapAdd) == 0 && len(svc.CapDrop) == 0 && svc.User == nil {
		return nil
	}
	result := &apiv1.SecurityContext{Capabilities: &apiv1.Capabilities{}}
	if len(svc.CapAdd) > 0 {
		result.Capabilities.Add = svc.CapAdd
	}
	if len(svc.CapDrop) > 0 {
		result.Capabilities.Drop = svc.CapDrop
	}
	if svc.User != nil {
		result.RunAsUser = svc.User.RunAsUser
		result.RunAsGroup = svc.User.RunAsGroup
	}
	return result
}

func translateStorageClass(className string) *string {
	if className != "" {
		return &className
	}
	return nil
}

func translateServiceEnvironment(svc *model.Service) []apiv1.EnvVar {
	result := []apiv1.EnvVar{}
	for _, e := range svc.Environment {
		if e.Name != "" {
			result = append(result, apiv1.EnvVar{Name: e.Name, Value: e.Value})
		}
	}
	return result
}

func translateContainerPorts(svc *model.Service) []apiv1.ContainerPort {
	result := []apiv1.ContainerPort{}
	sort.Slice(svc.Ports, func(i, j int) bool {
		return svc.Ports[i].ContainerPort < svc.Ports[j].ContainerPort
	})
	for _, p := range svc.Ports {
		result = append(result, apiv1.ContainerPort{ContainerPort: p.ContainerPort})
	}
	return result
}

func translateServicePorts(svc model.Service) []apiv1.ServicePort {
	result := []apiv1.ServicePort{}
	for _, p := range svc.Ports {
		if !isServicePortAdded(p.ContainerPort, result) {
			result = append(
				result,
				apiv1.ServicePort{
					Name:       fmt.Sprintf("p-%d-%d-%s", p.ContainerPort, p.ContainerPort, strings.ToLower(fmt.Sprintf("%v", p.Protocol))),
					Port:       p.ContainerPort,
					TargetPort: intstr.IntOrString{IntVal: p.ContainerPort},
					Protocol:   p.Protocol,
				},
			)
		}
		if p.HostPort != 0 && p.ContainerPort != p.HostPort && !isServicePortAdded(p.HostPort, result) {
			result = append(
				result,
				apiv1.ServicePort{
					Name:       fmt.Sprintf("p-%d-%d-%s", p.HostPort, p.ContainerPort, strings.ToLower(fmt.Sprintf("%v", p.Protocol))),
					Port:       p.HostPort,
					TargetPort: intstr.IntOrString{IntVal: p.ContainerPort},
					Protocol:   p.Protocol,
				},
			)
		}
	}
	return result
}

func isServicePortAdded(newPort int32, existentPorts []apiv1.ServicePort) bool {
	for _, p := range existentPorts {
		if p.Port == newPort {
			return true
		}
	}
	return false
}

func translateResources(svc *model.Service) apiv1.ResourceRequirements {
	result := apiv1.ResourceRequirements{}
	if svc.Resources != nil {
		if svc.Resources.Limits.CPU.Value.Cmp(resource.MustParse("0")) > 0 {
			result.Limits = apiv1.ResourceList{}
			result.Limits[apiv1.ResourceCPU] = svc.Resources.Limits.CPU.Value
		}

		if svc.Resources.Limits.Memory.Value.Cmp(resource.MustParse("0")) > 0 {
			if result.Limits == nil {
				result.Limits = apiv1.ResourceList{}
			}
			result.Limits[apiv1.ResourceMemory] = svc.Resources.Limits.Memory.Value
		}

		if svc.Resources.Requests.CPU.Value.Cmp(resource.MustParse("0")) > 0 {
			result.Requests = apiv1.ResourceList{}
			result.Requests[apiv1.ResourceCPU] = svc.Resources.Requests.CPU.Value
		}
		if svc.Resources.Requests.Memory.Value.Cmp(resource.MustParse("0")) > 0 {
			if result.Requests == nil {
				result.Requests = apiv1.ResourceList{}
			}
			result.Requests[apiv1.ResourceMemory] = svc.Resources.Requests.Memory.Value
		}
	}
	return result
}

type healthcheckProbes struct {
	readiness *apiv1.Probe
	liveness  *apiv1.Probe
}

func getSvcHealthProbe(svc *model.Service) healthcheckProbes {
	result := healthcheckProbes{}
	if svc.Healtcheck != nil {
		var handler apiv1.ProbeHandler
		if len(svc.Healtcheck.Test) != 0 {
			handler = apiv1.ProbeHandler{
				Exec: &apiv1.ExecAction{
					Command: svc.Healtcheck.Test,
				},
			}
		} else {
			handler = apiv1.ProbeHandler{
				HTTPGet: &apiv1.HTTPGetAction{
					Path: svc.Healtcheck.HTTP.Path,
					Port: intstr.IntOrString{IntVal: svc.Healtcheck.HTTP.Port},
				},
			}
		}
		probe := &apiv1.Probe{
			ProbeHandler:        handler,
			TimeoutSeconds:      int32(svc.Healtcheck.Timeout.Seconds()),
			PeriodSeconds:       int32(svc.Healtcheck.Interval.Seconds()),
			FailureThreshold:    int32(svc.Healtcheck.Retries),
			InitialDelaySeconds: int32(svc.Healtcheck.StartPeriod.Seconds()),
		}

		if svc.Healtcheck.Readiness {
			result.readiness = probe
		}
		if svc.Healtcheck.Liveness {
			result.liveness = probe
		}
	}
	return result
}

type updateStrategyGetter interface {
	validate(updateStrategy) error
	getDefault() updateStrategy
}

type deploymentStrategyGetter struct{}

func (*deploymentStrategyGetter) validate(updateStrategy updateStrategy) error {
	if updateStrategy != rollingUpdateStrategy && updateStrategy != recreateUpdateStrategy {
		return fmt.Errorf("invalid deployment update strategy: '%s'", updateStrategy)
	}
	return nil
}

func (*deploymentStrategyGetter) getDefault() updateStrategy {
	return recreateUpdateStrategy
}

type statefulSetStrategyGetter struct{}

func (*statefulSetStrategyGetter) validate(updateStrategy updateStrategy) error {
	if updateStrategy != rollingUpdateStrategy && updateStrategy != onDeleteUpdateStrategy {
		return fmt.Errorf("invalid statefulset update strategy: '%s'", updateStrategy)
	}
	return nil
}

func (*statefulSetStrategyGetter) getDefault() updateStrategy {
	return rollingUpdateStrategy
}

func getDeploymentUpdateStrategy(svc *model.Service) appsv1.DeploymentStrategy {
	result := getUpdateStrategy(svc, &deploymentStrategyGetter{})
	if result == rollingUpdateStrategy {
		return appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		}
	}
	return appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}
}

func getStatefulsetUpdateStrategy(svc *model.Service) appsv1.StatefulSetUpdateStrategy {
	result := getUpdateStrategy(svc, &statefulSetStrategyGetter{})
	if result == rollingUpdateStrategy {
		return appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
	}
	return appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.OnDeleteStatefulSetStrategyType,
	}
}

func getUpdateStrategy(svc *model.Service, strategy updateStrategyGetter) updateStrategy {
	if result := getUpdateStrategyByAnnotation(svc); result != "" {
		err := strategy.validate(result)
		if err == nil {
			return result
		}
		oktetoLog.Debugf("invalid strategy: %s", err)
	}
	if result := getUpdateStrategyByEnvVar(); result != "" {
		err := strategy.validate(result)
		if err == nil {
			return result
		}
		oktetoLog.Debugf("invalid strategy: %s", err)
	}
	return strategy.getDefault()
}

func getUpdateStrategyByAnnotation(svc *model.Service) updateStrategy {
	if v, ok := svc.Annotations[model.OktetoComposeUpdateStrategyAnnotation]; ok {
		return updateStrategy(v)
	}
	return ""
}

func getUpdateStrategyByEnvVar() updateStrategy {
	if v := os.Getenv(model.OktetoComposeUpdateStrategyEnvVar); v != "" {
		return updateStrategy(v)
	}
	return ""
}
