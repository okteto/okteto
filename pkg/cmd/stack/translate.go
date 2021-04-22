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
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/errors"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/subosito/gotenv"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	helmDriver = "secrets"

	nameField   = "name"
	statusField = "status"
	yamlField   = "yaml"
	outputField = "output"

	progressingStatus = "progressing"
	deployedStatus    = "deployed"
	errorStatus       = "error"
	destroyingStatus  = "destroying"

	pvcName = "pvc"
)

func translate(ctx context.Context, s *model.Stack, forceBuild, noCache bool) error {
	if err := translateStackEnvVars(s); err != nil {
		return err
	}

	return translateBuildImages(ctx, s, forceBuild, noCache)
}

func translateStackEnvVars(s *model.Stack) error {
	var err error
	for _, svc := range s.Services {
		svc.Image, err = model.ExpandEnv(svc.Image)
		if err != nil {
			return err
		}
		for _, envFilepath := range svc.EnvFiles {
			if err := translateServiceEnvFile(svc, envFilepath); err != nil {
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

func translateServiceEnvFile(svc *model.Service, filename string) error {
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

	envMap, err := gotenv.StrictParse(f)
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

func translateBuildImages(ctx context.Context, s *model.Stack, forceBuild, noCache bool) error {
	buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
	if err != nil {
		return err
	}
	building := false

	for name, svc := range s.Services {
		if svc.Build == nil {
			continue
		}
		if !isOktetoCluster && svc.Image == "" {
			return fmt.Errorf("'build' and 'image' fields of service '%s' cannot be empty", name)
		}
		if isOktetoCluster && !strings.HasPrefix(svc.Image, "okteto.dev") {
			svc.Image = fmt.Sprintf("okteto.dev/%s-%s:okteto", s.Name, name)
		}
		if !forceBuild {
			if _, err := registry.GetImageTagWithDigest(ctx, s.Namespace, svc.Image); err != errors.ErrNotFound {
				s.Services[name] = svc
				continue
			}
			log.Infof("image '%s' not found, building it", svc.Image)
		}
		if !building {
			building = true
			log.Information("Running your build in %s...", buildKitHost)
		}
		log.Information("Building image for service '%s'...", name)
		buildArgs := model.SerializeBuildArgs(svc.Build.Args)
		if err := build.Run(ctx, s.Namespace, buildKitHost, isOktetoCluster, svc.Build.Context, svc.Build.Dockerfile, svc.Image, svc.Build.Target, noCache, svc.Build.CacheFrom, buildArgs, nil, "tty"); err != nil {
			return fmt.Errorf("error building image for '%s': %s", name, err)
		}
		svc.SetLastBuiltAnnotation()
		s.Services[name] = svc
		log.Success("Image for service '%s' successfully pushed", name)
	}

	if !building && forceBuild {
		log.Warning("Ignoring '--build' argument. There are not 'build' primitives in your stack")
	}

	return nil
}

func translateConfigMap(s *model.Stack) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.GetConfigMapName(),
			Labels: map[string]string{
				okLabels.StackLabel: "true",
			},
		},
		Data: map[string]string{
			nameField: s.Name,
			yamlField: base64.StdEncoding.EncodeToString(s.Manifest),
		},
	}
}

func translateDeployment(svcName string, s *model.Stack) *appsv1.Deployment {
	svc := s.Services[svcName]
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
							WorkingDir:      svc.WorkingDir,
						},
					},
				},
			},
		},
	}
}

func translatePersistentVolumeClaims(name string, s *model.Stack) []apiv1.PersistentVolumeClaim {
	svc := s.Services[name]
	result := make([]apiv1.PersistentVolumeClaim, 0)
	for _, volume := range svc.Volumes {
		if volume.LocalPath == "" {
			continue
		}
		volumeSpec := s.Volumes[volume.LocalPath]
		labels := translateVolumeLabels(volume.LocalPath, s)
		annotations := translateAnnotations(svc)
		for key, value := range volumeSpec.Annotations {
			annotations[key] = value
		}

		pvcName := getVolumeClaimName(&volume)
		pvc := apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        pvcName,
				Namespace:   s.Namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: apiv1.PersistentVolumeClaimSpec{
				AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						"storage": volumeSpec.Storage.Size.Value,
					},
				},
				StorageClassName: translateStorageClass(volumeSpec.Storage.Class),
			},
		}
		result = append(result, pvc)
	}

	return result
}

func translateStatefulSet(name string, s *model.Stack) *appsv1.StatefulSet {
	svc := s.Services[name]

	initContainerCommand, initContainerVolumeMounts := getInitContainerCommandAndVolumeMounts(*svc)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   s.Namespace,
			Labels:      translateLabels(name, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             pointer.Int32Ptr(svc.Replicas),
			RevisionHistoryLimit: pointer.Int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: translateLabelSelector(name, s),
			},
			ServiceName: name,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(name, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: pointer.Int64Ptr(svc.StopGracePeriod),
					InitContainers: []apiv1.Container{
						{
							Name:         fmt.Sprintf("init-%s", name),
							Image:        "busybox",
							Command:      initContainerCommand,
							VolumeMounts: initContainerVolumeMounts,
						},
					},
					Containers: []apiv1.Container{
						{
							Name:            name,
							Image:           svc.Image,
							Command:         svc.Entrypoint.Values,
							Args:            svc.Command.Values,
							Env:             translateServiceEnvironment(svc),
							Ports:           translateContainerPorts(svc),
							SecurityContext: translateSecurityContext(svc),
							VolumeMounts:    translateVolumeMounts(name, svc),
							Resources:       translateResources(svc),
							WorkingDir:      svc.WorkingDir,
						},
					},
					Volumes: translateVolumes(name, svc),
				},
			},
			VolumeClaimTemplates: translateVolumeClaimTemplates(name, s),
		},
	}
}

func getInitContainerCommandAndVolumeMounts(svc model.Service) ([]string, []apiv1.VolumeMount) {
	volumeMounts := make([]apiv1.VolumeMount, 0)
	base := "chmod -R 777"
	var command string
	var addedVolumesVolume, addedDataVolume bool
	for _, volume := range svc.Volumes {
		volumeName := getVolumeClaimName(&volume)
		if volumeName != pvcName {
			volumeMounts = append(volumeMounts, apiv1.VolumeMount{Name: volumeName, MountPath: fmt.Sprintf("/volumes/%s", volumeName)})
			if !addedVolumesVolume {
				if command == "" {
					command = fmt.Sprintf("%s /volumes", base)
					addedVolumesVolume = true
				} else {
					command += fmt.Sprintf(" && %s /volumes", base)
				}
			}

		} else {
			if !addedDataVolume {
				volumeMounts = append(volumeMounts, apiv1.VolumeMount{Name: volumeName, MountPath: "/data"})
				if command == "" {
					command = fmt.Sprintf("%s /data", base)
					addedDataVolume = true
				} else {
					command += fmt.Sprintf(" && %s /data", base)
				}
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
	if s.Services[svcName].Public {
		annotations[okLabels.OktetoAutoIngressAnnotation] = "true"
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

func translateIngress(ingressName string, s *model.Stack) *extensions.Ingress {
	endpoints := s.Endpoints[ingressName]
	return &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   s.Namespace,
			Labels:      translateIngressLabels(ingressName, s),
			Annotations: translateIngressAnnotations(ingressName, s),
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: translateEndpoints(endpoints),
						},
					},
				},
			},
		},
	}
}

func translateEndpoints(endpoints model.Endpoint) []extensions.HTTPIngressPath {
	paths := make([]extensions.HTTPIngressPath, 0)
	for _, rule := range endpoints.Rules {
		path := extensions.HTTPIngressPath{
			Path: rule.Path,
			Backend: extensions.IngressBackend{
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
	annotations := map[string]string{okLabels.OktetoIngressAutoGenerateHost: "true"}
	for k := range endpoint.Annotations {
		annotations[k] = endpoint.Annotations[k]
	}
	return annotations
}

func translateIngressLabels(endpointName string, s *model.Stack) map[string]string {
	endpoint := s.Endpoints[endpointName]
	labels := map[string]string{
		okLabels.StackNameLabel:         s.Name,
		okLabels.StackEndpointNameLabel: endpointName,
	}
	for k := range endpoint.Labels {
		labels[k] = endpoint.Labels[k]
	}
	return labels
}

func translateVolumeLabels(volumeName string, s *model.Stack) map[string]string {
	volume := s.Volumes[volumeName]
	labels := map[string]string{
		okLabels.StackNameLabel:       s.Name,
		okLabels.StackVolumeNameLabel: volumeName,
	}
	for k := range volume.Labels {
		labels[k] = volume.Labels[k]
	}
	return labels
}

func translateLabels(svcName string, s *model.Stack) map[string]string {
	svc := s.Services[svcName]
	labels := map[string]string{
		okLabels.StackNameLabel:        s.Name,
		okLabels.StackServiceNameLabel: svcName,
	}
	for k := range svc.Labels {
		labels[k] = svc.Labels[k]
	}
	return labels
}

func translateLabelSelector(svcName string, s *model.Stack) map[string]string {
	labels := map[string]string{
		okLabels.StackNameLabel:        s.Name,
		okLabels.StackServiceNameLabel: svcName,
	}
	return labels
}

func translateAnnotations(svc *model.Service) map[string]string {
	result := map[string]string{}
	for k, v := range svc.Annotations {
		result[k] = v
	}
	return result
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
	if len(svc.CapAdd) == 0 && len(svc.CapDrop) == 0 {
		return nil
	}
	result := &apiv1.SecurityContext{Capabilities: &apiv1.Capabilities{}}
	if len(svc.CapAdd) > 0 {
		result.Capabilities.Add = svc.CapAdd
	}
	if len(svc.CapDrop) > 0 {
		result.Capabilities.Drop = svc.CapDrop
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
		result = append(result, apiv1.EnvVar{Name: e.Name, Value: e.Value})
	}
	return result
}

func translateContainerPorts(svc *model.Service) []apiv1.ContainerPort {
	result := []apiv1.ContainerPort{}
	for _, p := range svc.Ports {
		result = append(result, apiv1.ContainerPort{ContainerPort: p.Port})
	}
	return result
}

func translateServicePorts(svc model.Service) []apiv1.ServicePort {
	result := []apiv1.ServicePort{}
	for _, p := range svc.Ports {
		result = append(
			result,
			apiv1.ServicePort{
				Name:       fmt.Sprintf("p-%d-%s", p.Port, strings.ToLower(fmt.Sprintf("%v", p.Protocol))),
				Port:       int32(p.Port),
				TargetPort: intstr.IntOrString{IntVal: p.Port},
				Protocol:   p.Protocol,
			},
		)
	}
	return result
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
