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
	"github.com/okteto/okteto/pkg/k8s/client"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/subosito/gotenv"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
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
	for name, svc := range s.Services {
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
		s.Services[name] = svc
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
	c, _, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if s.Namespace == "" {
		s.Namespace = client.GetContextNamespace("")
	}

	oktetoRegistryURL := ""
	n, err := namespaces.Get(ctx, s.Namespace, c)
	if err == nil {
		if namespaces.IsOktetoNamespace(n) {
			oktetoRegistryURL, err = okteto.GetRegistry()
			if err != nil {
				return err
			}
		}
	}

	buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
	if err != nil {
		return err
	}
	building := false

	for name, svc := range s.Services {
		if svc.Build == nil {
			continue
		}
		if !forceBuild {
			if _, err := registry.GetImageTagWithDigest(ctx, s.Namespace, svc.Image); err != errors.ErrNotFound {
				continue
			}
			log.Infof("image '%s' not found, building it", svc.Image)
		}
		if !building {
			building = true
			log.Information("Running your build in %s...", buildKitHost)
		}
		imageTag := registry.GetImageTag(svc.Image, name, s.Namespace, oktetoRegistryURL)
		log.Information("Building image for service '%s'...", name)
		buildArgs := model.SerializeBuildArgs(svc.Build.Args)
		if err := build.Run(ctx, s.Namespace, buildKitHost, isOktetoCluster, svc.Build.Context, svc.Build.Dockerfile, imageTag, svc.Build.Target, noCache, svc.Build.CacheFrom, buildArgs, "tty"); err != nil {
			return fmt.Errorf("error building image for '%s': %s", name, err)
		}
		svc.Image = imageTag
		svc.SetLastBuiltAnnotationtamp()
		s.Services[name] = svc
		log.Success("Image for service '%s' successfully pushed", name)
	}

	return nil
}

func translateConfigMap(s *model.Stack) *apiv1.ConfigMap {
	marshalled, err := yaml.Marshal(s)
	if err != nil {
		log.Errorf("error marshalling stack: %s", err.Error())
	}
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.GetConfigMapName(),
			Labels: map[string]string{
				okLabels.StackLabel: "true",
			},
		},
		Data: map[string]string{
			nameField: s.Name,
			yamlField: base64.StdEncoding.EncodeToString(marshalled),
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
			Replicas: pointer.Int32Ptr(svc.Deploy.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: translateLabelSelector(svcName, s),
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      translateLabels(svcName, s),
					Annotations: translateAnnotations(svc),
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: pointer.Int64Ptr(int64(svc.StopGracePeriod.Seconds())),
					Containers: []apiv1.Container{
						{
							Name:            svcName,
							Image:           svc.Image,
							Command:         svc.Entrypoint.Values,
							Args:            svc.Command.Values,
							Env:             translateServiceEnvironment(svc),
							Ports:           translateContainerPorts(svc),
						},
					},
				},
			},
		},
	}
}

func translateStatefulSet(name string, s *model.Stack) *appsv1.StatefulSet {
	svc := s.Services[name]
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   s.Namespace,
			Labels:      translateLabels(name, s),
			Annotations: translateAnnotations(svc),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             pointer.Int32Ptr(svc.Deploy.Replicas),
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
					TerminationGracePeriodSeconds: pointer.Int64Ptr(int64(svc.StopGracePeriod.Seconds())),
					InitContainers: []apiv1.Container{
						{
							Name:    fmt.Sprintf("init-%s", name),
							Image:   "busybox",
							Command: []string{"chmod", "-R", "777", "/data"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									MountPath: "/data",
									Name:      pvcName,
								},
							},
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
						},
					},
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        pvcName,
						Labels:      translateLabels(name, s),
						Annotations: translateAnnotations(svc),
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								"storage": svc.Resources.Storage.Size.Value,
							},
						},
						StorageClassName: translateStorageClass(svc),
					},
				},
			},
		},
	}
}

func translateService(svcName string, s *model.Stack) *apiv1.Service {
	svc := s.Services[svcName]
	annotations := translateAnnotations(svc)
	if svc.Public {
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
			Type:     translateServiceType(svc),
			Ports:    translateServicePorts(svc),
		},
	}
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

func translateServiceType(svc *model.Service) apiv1.ServiceType {
	if svc.Public {
		return apiv1.ServiceTypeLoadBalancer
	}
	return apiv1.ServiceTypeClusterIP
}

func translateVolumeMounts(svc *model.Service) []apiv1.VolumeMount {
	result := []apiv1.VolumeMount{}
	for i, v := range svc.Volumes {
		result = append(
			result,
			apiv1.VolumeMount{
				MountPath: v.RemotePath,
				Name:      pvcName,
				SubPath:   fmt.Sprintf("data-%d", i),
			},
		)
	}
	return result
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

func translateStorageClass(svc *model.Service) *string {
	if svc.Resources.Storage.Class != "" {
		return &svc.Resources.Storage.Class
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

func translateServicePorts(svc *model.Service) []apiv1.ServicePort {
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
	if svc.Resources.CPU.Value.Cmp(resource.MustParse("0")) > 0 {
		result.Limits = apiv1.ResourceList{}
		result.Limits[apiv1.ResourceCPU] = svc.Resources.CPU.Value
	}
	if svc.Resources.Memory.Value.Cmp(resource.MustParse("0")) > 0 {
		if result.Limits == nil {
			result.Limits = apiv1.ResourceList{}
		}
		result.Limits[apiv1.ResourceMemory] = svc.Resources.Memory.Value
	}
	return result
}
