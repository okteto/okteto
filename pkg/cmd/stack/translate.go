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
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/godotenv"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
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
)

func translate(ctx context.Context, s *model.Stack, options *StackDeployOptions) error {
	if err := translateStackEnvVars(ctx, s); err != nil {
		return err
	}

	return translateBuildImages(ctx, s, options)
}

func translateStackEnvVars(ctx context.Context, s *model.Stack) error {
	for svcName, svc := range s.Services {
		for i := len(svc.EnvFiles) - 1; i >= 0; i-- {
			envFilepath := svc.EnvFiles[i]
			if err := translateServiceEnvFile(ctx, svc, svcName, envFilepath); err != nil {
				if filepath.Base(envFilepath) == ".env" {
					oktetoLog.Warning("Skipping '.env' file from %s service", svcName)
					continue
				}
				return err
			}
		}
		sort.SliceStable(svc.Environment, func(i, j int) bool {
			return strings.Compare(svc.Environment[i].Name, svc.Environment[j].Name) < 0
		})
		svc.EnvFiles = nil
	}
	return nil
}

func translateServiceEnvFile(ctx context.Context, svc *model.Service, svcName, filename string) error {
	var err error
	filename, err = model.ExpandEnv(filename)
	if err != nil {
		return err
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	envMap, err := godotenv.ParseWithLookup(f, os.LookupEnv)
	if err != nil {
		return fmt.Errorf("error parsing env_file %s: %s", filename, err.Error())
	}

	for _, e := range svc.Environment {
		delete(envMap, e.Name)
	}

	for name, value := range envMap {
		svc.Environment = append(
			svc.Environment,
			model.EnvVar{Name: name, Value: value},
		)
	}

	return nil
}

func translateBuildImages(ctx context.Context, s *model.Stack, options *StackDeployOptions) error {
	hasBuiltSomething, err := buildServices(ctx, s, options)
	if err != nil {
		return err
	}
	var hasAddedAnyVolumeMounts bool
	if okteto.IsOkteto() {
		hasAddedAnyVolumeMounts, err = addVolumeMountsToBuiltImage(ctx, s, options, hasBuiltSomething)
		if err != nil {
			return err
		}
	}

	if !hasBuiltSomething && !hasAddedAnyVolumeMounts && options.ForceBuild {
		oktetoLog.Warning("Ignoring '--build' argument. There are not 'build' primitives in your stack")
	}

	return nil
}

func buildServices(ctx context.Context, s *model.Stack, options *StackDeployOptions) (bool, error) {
	hasBuiltSomething := false

	for _, name := range options.ServicesToDeploy {
		svc := s.Services[name]
		if svc.Build == nil {
			continue
		}
		if !okteto.IsOkteto() && svc.Image == "" {
			return hasBuiltSomething, fmt.Errorf("'build' and 'image' fields of service '%s' cannot be empty", name)
		}
		if okteto.IsOkteto() && !registry.IsOktetoRegistry(svc.Image) {
			svc.Image = fmt.Sprintf("okteto.dev/%s-%s:okteto", s.Name, name)
		}
		if !options.ForceBuild {
			if _, err := registry.GetImageTagWithDigest(svc.Image); err != oktetoErrors.ErrNotFound {
				s.Services[name] = svc
				continue
			}
			oktetoLog.Infof("image '%s' not found, building it", svc.Image)
		}
		if !hasBuiltSomething {
			hasBuiltSomething = true
			if okteto.Context().Builder != "" {
				oktetoLog.Information("Running your build in %s...", okteto.Context().Builder)
			} else {
				oktetoLog.Information("Running your build in docker")
			}

		}
		oktetoLog.Information("Building image for service '%s'...", name)
		buildArgs := model.SerializeBuildArgs(svc.Build.Args)
		buildOptions := build.BuildOptions{
			Path:       svc.Build.Context,
			File:       svc.Build.Dockerfile,
			Tag:        svc.Image,
			Target:     svc.Build.Target,
			NoCache:    options.NoCache,
			CacheFrom:  svc.Build.CacheFrom,
			BuildArgs:  buildArgs,
			OutputMode: options.Progress,
		}
		if err := build.Run(ctx, buildOptions); err != nil {
			return hasBuiltSomething, err
		}
		svc.SetLastBuiltAnnotation()
		s.Services[name] = svc
		oktetoLog.Success("Image for service '%s' successfully pushed", name)
	}
	return hasBuiltSomething, nil
}

func addVolumeMountsToBuiltImage(ctx context.Context, s *model.Stack, options *StackDeployOptions, hasBuiltSomething bool) (bool, error) {
	hasAddedAnyVolumeMounts := false
	for name, svc := range s.Services {
		notSkippableVolumeMounts := getAccessibleVolumeMounts(s, name)
		if len(notSkippableVolumeMounts) != 0 {
			fromImage := svc.Image
			if okteto.IsOkteto() {
				fromImage = registry.ExpandOktetoDevRegistry(svc.Image)
				fromImage = registry.ExpandOktetoGlobalRegistry(fromImage)
			}

			svcBuild, err := registry.CreateDockerfileWithVolumeMounts(fromImage, notSkippableVolumeMounts)
			if err != nil {
				return hasAddedAnyVolumeMounts, err
			}
			svc.Build = svcBuild
			if okteto.IsOkteto() {
				svc.Image = fmt.Sprintf("okteto.dev/%s-%s:okteto-with-volume-mounts", s.Name, name)
			}
			if !options.ForceBuild {
				if _, err := registry.GetImageTagWithDigest(svc.Image); err != oktetoErrors.ErrNotFound {
					s.Services[name] = svc
					continue
				}
				oktetoLog.Infof("image '%s' not found, building it", svc.Image)
			}
			if !hasBuiltSomething && !hasAddedAnyVolumeMounts {
				hasAddedAnyVolumeMounts = true
				oktetoLog.Information("Running your build in %s...", okteto.Context().Builder)
			}
			oktetoLog.Information("Building image for service '%s' to include host volumes...", name)
			buildArgs := model.SerializeBuildArgs(svc.Build.Args)

			buildOptions := build.BuildOptions{
				Path:       svc.Build.Context,
				File:       svc.Build.Dockerfile,
				Tag:        svc.Image,
				Target:     svc.Build.Target,
				NoCache:    options.NoCache,
				CacheFrom:  svc.Build.CacheFrom,
				BuildArgs:  buildArgs,
				OutputMode: options.Progress,
			}
			if err := build.Run(ctx, buildOptions); err != nil {
				return hasAddedAnyVolumeMounts, err
			}
			svc.SetLastBuiltAnnotation()
			s.Services[name] = svc
			oktetoLog.Success("Image for service '%s' successfully pushed", name)
		}
	}
	return hasAddedAnyVolumeMounts, nil
}

func getAccessibleVolumeMounts(stack *model.Stack, svcName string) []model.StackVolume {
	accessibleVolumeMounts := make([]model.StackVolume, 0)
	for _, volume := range stack.Services[svcName].VolumeMounts {
		if _, err := os.Stat(volume.LocalPath); !os.IsNotExist(err) {
			accessibleVolumeMounts = append(accessibleVolumeMounts, volume)
		} else {
			warning := fmt.Sprintf("[%s]: volume '%s:%s' will be ignored. Could not find '%s'.", svcName, volume.LocalPath, volume.RemotePath, volume.LocalPath)
			stack.Warnings.VolumeMountWarnings = append(stack.Warnings.VolumeMountWarnings, warning)
		}
	}
	return accessibleVolumeMounts
}

func translateConfigMap(s *model.Stack) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.GetStackConfigMapName(s.Name),
			Labels: map[string]string{
				model.StackLabel: "true",
			},
		},
		Data: map[string]string{
			NameField:    s.Name,
			YamlField:    base64.StdEncoding.EncodeToString(s.Manifest),
			ComposeField: strconv.FormatBool(s.IsCompose),
		},
	}
}

func translateDeployment(svcName string, s *model.Stack) *appsv1.Deployment {
	svc := s.Services[svcName]

	healthcheckProbe := getSvcProbe(svc)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(svc.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: translateLabelSelector(svcName, s),
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: pointer.Int64Ptr(svc.StopGracePeriod),
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
							ReadinessProbe:  healthcheckProbe,
							LivenessProbe:   healthcheckProbe,
						},
					},
				},
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
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					"storage": volumeSpec.Size.Value,
				},
			},
			StorageClassName: translateStorageClass(volumeSpec.Class),
		},
	}
	return pvc
}

func translateStatefulSet(svcName string, s *model.Stack) *appsv1.StatefulSet {
	svc := s.Services[svcName]

	initContainers := getInitContainers(svcName, s)
	healthcheckProbe := getSvcProbe(svc)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             pointer.Int32Ptr(svc.Replicas),
			RevisionHistoryLimit: pointer.Int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: translateLabelSelector(svcName, s),
			},
			ServiceName: svcName,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: pointer.Int64Ptr(svc.StopGracePeriod),
					InitContainers:                initContainers,
					Affinity:                      translateAffinity(svc),
					Volumes:                       translateVolumes(svcName, svc),
					Containers: []apiv1.Container{
						{
							Name:            svcName,
							Image:           svc.Image,
							Command:         svc.Entrypoint.Values,
							Args:            svc.Command.Values,
							Env:             translateServiceEnvironment(svc),
							Ports:           translateContainerPorts(svc),
							SecurityContext: translateSecurityContext(svc),
							VolumeMounts:    translateVolumeMounts(svcName, svc),
							Resources:       translateResources(svc),
							WorkingDir:      svc.Workdir,
							ReadinessProbe:  healthcheckProbe,
							LivenessProbe:   healthcheckProbe,
						},
					},
				},
			},
			VolumeClaimTemplates: translateVolumeClaimTemplates(svcName, s),
		},
	}
}

func translateJob(svcName string, s *model.Stack) *batchv1.Job {
	svc := s.Services[svcName]

	initContainers := getInitContainers(svcName, s)
	healthcheckProbe := getSvcProbe(svc)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: batchv1.JobSpec{
			Completions:  pointer.Int32Ptr(svc.Replicas),
			Parallelism:  pointer.Int32Ptr(1),
			BackoffLimit: &svc.BackOffLimit,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: apiv1.PodSpec{
					RestartPolicy:                 svc.RestartPolicy,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(svc.StopGracePeriod),
					InitContainers:                initContainers,
					Affinity:                      translateAffinity(svc),
					Containers: []apiv1.Container{
						{
							Name:            svcName,
							Image:           svc.Image,
							Command:         svc.Entrypoint.Values,
							Args:            svc.Command.Values,
							Env:             translateServiceEnvironment(svc),
							Ports:           translateContainerPorts(svc),
							SecurityContext: translateSecurityContext(svc),
							VolumeMounts:    translateVolumeMounts(svcName, svc),
							Resources:       translateResources(svc),
							WorkingDir:      svc.Workdir,
							ReadinessProbe:  healthcheckProbe,
							LivenessProbe:   healthcheckProbe,
						},
					},
					Volumes: translateVolumes(svcName, svc),
				},
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
		Name:         fmt.Sprintf("init-%s", svcName),
		Image:        "busybox",
		Command:      initContainerCommand,
		VolumeMounts: initContainerVolumeMounts,
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
						Resources: apiv1.ResourceRequirements{
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

func translateVolumes(svcName string, svc *model.Service) []apiv1.Volume {
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
	if s.Services[svcName].Public && annotations[model.OktetoAutoIngressAnnotation] == "" {
		annotations[model.OktetoAutoIngressAnnotation] = "true"
		if annotations[model.OktetoPrivateSvcAnnotation] == "true" {
			annotations[model.OktetoAutoIngressAnnotation] = "private"
		}
	}
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Labels:      translateLabels(svcName, s),
			Annotations: annotations,
		},
		Spec: apiv1.ServiceSpec{
			Selector: translateLabelSelector(svcName, s),
			Type:     translateServiceType(*svc),
			Ports:    translateServicePorts(*svc),
		},
	}
}

func translateIngressV1(ingressName string, s *model.Stack) *networkingv1.Ingress {
	endpoints := s.Endpoints[ingressName]
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   s.Namespace,
			Labels:      translateIngressLabels(ingressName, s),
			Annotations: translateIngressAnnotations(ingressName, s),
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: translateEndpointsV1(endpoints),
						},
					},
				},
			},
		},
	}
}

func translateIngressV1Beta1(ingressName string, s *model.Stack) *networkingv1beta1.Ingress {
	endpoints := s.Endpoints[ingressName]
	return &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   s.Namespace,
			Labels:      translateIngressLabels(ingressName, s),
			Annotations: translateIngressAnnotations(ingressName, s),
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: translateEndpointsV1Beta1(endpoints),
						},
					},
				},
			},
		},
	}
}

func translateEndpointsV1(endpoints model.Endpoint) []networkingv1.HTTPIngressPath {
	paths := make([]networkingv1.HTTPIngressPath, 0)
	pathType := networkingv1.PathTypeImplementationSpecific
	for _, rule := range endpoints.Rules {
		path := networkingv1.HTTPIngressPath{
			Path:     rule.Path,
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: rule.Service,
					Port: networkingv1.ServiceBackendPort{
						Number: rule.Port,
					},
				},
			},
		}
		paths = append(paths, path)
	}
	return paths
}

func translateEndpointsV1Beta1(endpoints model.Endpoint) []networkingv1beta1.HTTPIngressPath {
	paths := make([]networkingv1beta1.HTTPIngressPath, 0)
	for _, rule := range endpoints.Rules {
		path := networkingv1beta1.HTTPIngressPath{
			Path: rule.Path,
			Backend: networkingv1beta1.IngressBackend{
				ServiceName: rule.Service,
				ServicePort: intstr.IntOrString{IntVal: rule.Port},
			},
		}
		paths = append(paths, path)
	}
	return paths
}

func translateIngressAnnotations(endpointName string, s *model.Stack) map[string]string {
	endpoint := s.Endpoints[endpointName]
	annotations := model.Annotations{model.OktetoIngressAutoGenerateHost: "true"}
	for k := range endpoint.Annotations {
		annotations[k] = endpoint.Annotations[k]
	}
	return annotations
}

func translateIngressLabels(endpointName string, s *model.Stack) map[string]string {
	endpoint := s.Endpoints[endpointName]
	labels := map[string]string{
		model.StackNameLabel:         s.Name,
		model.StackEndpointNameLabel: endpointName,
	}
	for k := range endpoint.Labels {
		labels[k] = endpoint.Labels[k]
	}
	return labels
}

func translateVolumeLabels(volumeName string, s *model.Stack) map[string]string {
	volume := s.Volumes[volumeName]
	labels := map[string]string{
		model.StackNameLabel:       s.Name,
		model.StackVolumeNameLabel: volumeName,
	}
	for k := range volume.Labels {
		labels[k] = volume.Labels[k]
	}
	return labels
}

func translateAffinity(svc *model.Service) *apiv1.Affinity {
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
		model.StackNameLabel:        s.Name,
		model.StackServiceNameLabel: svcName,
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
		model.StackNameLabel:        s.Name,
		model.StackServiceNameLabel: svcName,
	}
	return labels
}

func translateAnnotations(svc *model.Service) map[string]string {

	result := getAnnotations()
	for k, v := range svc.Annotations {
		result[k] = v
	}
	return result
}

func getAnnotations() map[string]string {
	annotations := map[string]string{}
	if utils.IsOktetoRepo() {
		annotations[model.OktetoSampleAnnotation] = "true"
	}
	return annotations
}

func translateServiceType(svc model.Service) apiv1.ServiceType {
	if svc.Public {
		return apiv1.ServiceTypeLoadBalancer
	}
	return apiv1.ServiceTypeClusterIP
}

func translateVolumeMounts(svcName string, svc *model.Service) []apiv1.VolumeMount {
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

func getVolumeClaimName(v *model.StackVolume) string {
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
					Port:       int32(p.ContainerPort),
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
					Port:       int32(p.HostPort),
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
				result.Requests[apiv1.ResourceMemory] = svc.Resources.Requests.Memory.Value
			}
		}
	}
	return result
}

func getSvcProbe(svc *model.Service) *apiv1.Probe {
	if svc.Healtcheck != nil {
		var handler apiv1.Handler
		if len(svc.Healtcheck.Test) != 0 {
			handler = apiv1.Handler{
				Exec: &apiv1.ExecAction{
					Command: svc.Healtcheck.Test,
				},
			}
		} else {
			handler = apiv1.Handler{
				HTTPGet: &apiv1.HTTPGetAction{
					Path: svc.Healtcheck.HTTP.Path,
					Port: intstr.IntOrString{IntVal: svc.Healtcheck.HTTP.Port},
				},
			}
		}
		return &apiv1.Probe{
			Handler:             handler,
			TimeoutSeconds:      int32(svc.Healtcheck.Timeout.Seconds()),
			PeriodSeconds:       int32(svc.Healtcheck.Interval.Seconds()),
			FailureThreshold:    int32(svc.Healtcheck.Retries),
			InitialDelaySeconds: int32(svc.Healtcheck.StartPeriod.Seconds()),
		}
	}
	return nil
}
