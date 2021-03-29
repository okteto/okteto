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
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	env = `A=1
# comment


B=$B

C=3`
)

func Test_translate(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Name: "name",
		Services: map[string]*model.Service{
			"1": {
				Image:    "image",
				EnvFiles: []string{"/non-existing"},
			},
		},
	}
	if err := translate(ctx, stack, false, false); err == nil {
		t.Fatalf("An error should be returned")
	}
}

func Test_translateEnvVars(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", ".env")
	if err != nil {
		t.Fatalf("failed to create dynamic env file: %s", err.Error())
	}
	if err := ioutil.WriteFile(tmpFile.Name(), []byte(env), 0600); err != nil {
		t.Fatalf("failed to write env file: %s", err.Error())
	}
	defer os.RemoveAll(tmpFile.Name())

	os.Setenv("IMAGE", "image")
	os.Setenv("B", "2")
	os.Setenv("ENV_PATH", tmpFile.Name())
	stack := &model.Stack{
		Name: "name",
		Services: map[string]*model.Service{
			"1": {
				Image:    "${IMAGE}",
				EnvFiles: []string{"${ENV_PATH}"},
				Environment: []model.EnvVar{
					{
						Name:  "C",
						Value: "original",
					},
				},
			},
		},
	}
	translateStackEnvVars(stack)
	if stack.Services["1"].Image != "image" {
		t.Errorf("Wrong image: %s", stack.Services["1"].Image)
	}
	if len(stack.Services["1"].Environment) != 3 {
		t.Errorf("Wrong envirironment: %v", stack.Services["1"].Environment)
	}
	for _, e := range stack.Services["1"].Environment {
		if e.Name == "A" && e.Value != "1" {
			t.Errorf("Wrong envirironment variable A: %s", e.Value)
		}
		if e.Name == "B" && e.Value != "2" {
			t.Errorf("Wrong envirironment variable B: %s", e.Value)
		}
		if e.Name == "C" && e.Value != "original" {
			t.Errorf("Wrong envirironment variable C: %s", e.Value)
		}
	}
}

func Test_translateConfigMap(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Image: "image",
			},
		},
	}
	result := translateConfigMap(s)
	if result.Name != "okteto-stackName" {
		t.Errorf("Wrong configmap name: '%s'", result.Name)
	}
	if result.Labels[okLabels.StackLabel] != "true" {
		t.Errorf("Wrong labels: '%s'", result.Labels)
	}
	if result.Data[nameField] != "stackName" {
		t.Errorf("Wrong data.name: '%s'", result.Data[nameField])
	}
	if result.Data[yamlField] != "bmFtZTogc3RhY2tOYW1lCnNlcnZpY2VzOgogIHN2Y05hbWU6CiAgICBpbWFnZTogaW1hZ2UKd2FybmluZ3M6IFtdCg==" {
		t.Errorf("Wrong data.yaml: '%s'", result.Data[yamlField])
	}
}

func Test_translateDeployment(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Image:           "image",
				Deploy:          &model.DeployInfo{Replicas: 3},
				StopGracePeriod: 20,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []model.EnvVar{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports: []model.Port{{Port: 80}, {Port: 90}},
			},
		},
	}
	result := translateDeployment("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong deployment name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                       "value1",
		"label2":                       "value2",
		okLabels.StackNameLabel:        "stackName",
		okLabels.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong deployment labels: '%s'", result.Labels)
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong deployment annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Replicas != 3 {
		t.Errorf("Wrong deployment spec.replicas: '%d'", *result.Spec.Replicas)
	}
	selector := map[string]string{
		okLabels.StackNameLabel:        "stackName",
		okLabels.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Spec.Selector.MatchLabels, selector) {
		t.Errorf("Wrong spec.selector: '%s'", result.Spec.Selector.MatchLabels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong deployment spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong deployment container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong deployment container.image: '%s'", c.Image)
	}
	if !reflect.DeepEqual(c.Command, []string{"command1", "command2"}) {
		t.Errorf("Wrong container.command: '%v'", c.Command)
	}
	if !reflect.DeepEqual(c.Args, []string{"args1", "args2"}) {
		t.Errorf("Wrong container.args: '%v'", c.Args)
	}
	env := []apiv1.EnvVar{{Name: "env1", Value: "value1"}, {Name: "env2", Value: "value2"}}
	if !reflect.DeepEqual(c.Env, env) {
		t.Errorf("Wrong container.env: '%v'", c.Env)
	}
	ports := []apiv1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 90}}
	if !reflect.DeepEqual(c.Ports, ports) {
		t.Errorf("Wrong container.ports: '%v'", c.Ports)
	}
	if c.SecurityContext != nil {
		t.Errorf("Wrong deployment container.security_context: '%v'", c.SecurityContext)
	}
	if !reflect.DeepEqual(c.Resources, apiv1.ResourceRequirements{}) {
		t.Errorf("Wrong container.resources: '%v'", c.Resources)
	}
}

func Test_translateStatefulSet(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Image:           "image",
				Deploy:          &model.DeployInfo{Replicas: 3, Resources: model.ResourceRequirements{Limits: map[apiv1.ResourceName]resource.Quantity{apiv1.ResourceCPU: resource.MustParse("100m"), apiv1.ResourceMemory: resource.MustParse("1Gi")}}},
				StopGracePeriod: 20,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []model.EnvVar{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:   []model.Port{{Port: 80}, {Port: 90}},
				CapAdd:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},
				Volumes: []model.VolumeStack{{RemotePath: "/volume1"}, {RemotePath: "/volume2"}},
				Resources: model.ServiceResources{

					Storage: model.StorageResource{
						Size:  model.Quantity{Value: resource.MustParse("20Gi")},
						Class: "class-name",
					},
				},
			},
		},
	}
	result := translateStatefulSet("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong statefulset name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                       "value1",
		"label2":                       "value2",
		okLabels.StackNameLabel:        "stackName",
		okLabels.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong statefulset labels: '%s'", result.Labels)
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong statefulset annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Replicas != 3 {
		t.Errorf("Wrong statefulset spec.replicas: '%d'", *result.Spec.Replicas)
	}
	selector := map[string]string{
		okLabels.StackNameLabel:        "stackName",
		okLabels.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Spec.Selector.MatchLabels, selector) {
		t.Errorf("Wrong spec.selector: '%s'", result.Spec.Selector.MatchLabels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong statefulset spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	initContainer := apiv1.Container{
		Name:    fmt.Sprintf("init-%s", "svcName"),
		Image:   "busybox",
		Command: []string{"chmod", "-R", "777", "/data"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/data",
				Name:      pvcName,
			},
		},
	}
	if !reflect.DeepEqual(result.Spec.Template.Spec.InitContainers[0], initContainer) {
		t.Errorf("Wrong statefulset init container: '%v'", result.Spec.Template.Spec.InitContainers[0])
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong statefulset container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong statefulset container.image: '%s'", c.Image)
	}
	if !reflect.DeepEqual(c.Command, []string{"command1", "command2"}) {
		t.Errorf("Wrong container.command: '%v'", c.Command)
	}
	if !reflect.DeepEqual(c.Args, []string{"args1", "args2"}) {
		t.Errorf("Wrong container.args: '%v'", c.Args)
	}
	env := []apiv1.EnvVar{{Name: "env1", Value: "value1"}, {Name: "env2", Value: "value2"}}
	if !reflect.DeepEqual(c.Env, env) {
		t.Errorf("Wrong container.env: '%v'", c.Env)
	}
	ports := []apiv1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 90}}
	if !reflect.DeepEqual(c.Ports, ports) {
		t.Errorf("Wrong container.ports: '%v'", c.Ports)
	}
	securityContext := apiv1.SecurityContext{
		Capabilities: &apiv1.Capabilities{
			Add:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
			Drop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},
		},
	}
	if !reflect.DeepEqual(*c.SecurityContext, securityContext) {
		t.Errorf("Wrong statefulset container.security_context: '%v'", c.SecurityContext)
	}
	resources := apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse("100m"),
			apiv1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	if !reflect.DeepEqual(c.Resources, resources) {
		t.Errorf("Wrong container.resources: '%v'", c.Resources)
	}
	volumeMounts := []apiv1.VolumeMount{
		{
			MountPath: "/volume1",
			Name:      pvcName,
			SubPath:   "data-0",
		},
		{
			MountPath: "/volume2",
			Name:      pvcName,
			SubPath:   "data-1",
		},
	}
	if !reflect.DeepEqual(c.VolumeMounts, volumeMounts) {
		t.Errorf("Wrong container.volume_mounts: '%v'", c.VolumeMounts)
	}

	vct := result.Spec.VolumeClaimTemplates[0]
	if vct.Name != pvcName {
		t.Errorf("Wrong statefulset name: '%s'", vct.Name)
	}
	if !reflect.DeepEqual(vct.Labels, labels) {
		t.Errorf("Wrong statefulset labels: '%s'", vct.Labels)
	}
	if !reflect.DeepEqual(vct.Annotations, annotations) {
		t.Errorf("Wrong statefulset annotations: '%s'", vct.Annotations)
	}
	volumeClaimTemplateSpec := apiv1.PersistentVolumeClaimSpec{
		AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
		Resources: apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				"storage": resource.MustParse("20Gi"),
			},
		},
		StorageClassName: pointer.StringPtr("class-name"),
	}
	if !reflect.DeepEqual(vct.Spec, volumeClaimTemplateSpec) {
		t.Errorf("Wrong statefulset volume claim template: '%v'", vct.Spec)
	}
}

func Test_translateService(t *testing.T) {
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				Ports: []model.Port{{Port: 80, Protocol: apiv1.ProtocolTCP}, {Port: 90, Protocol: apiv1.ProtocolTCP}},
			},
		},
	}
	result := translateService("svcName", s)
	if result.Name != "svcName" {
		t.Errorf("Wrong service name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                       "value1",
		"label2":                       "value2",
		okLabels.StackNameLabel:        "stackName",
		okLabels.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Labels, labels) {
		t.Errorf("Wrong service labels: '%s'", result.Labels)
	}
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong service annotations: '%s'", result.Annotations)
	}
	ports := []apiv1.ServicePort{
		{
			Name:       "p-80-tcp",
			Port:       80,
			TargetPort: intstr.IntOrString{IntVal: 80},
			Protocol:   apiv1.ProtocolTCP,
		},
		{
			Name:       "p-90-tcp",
			Port:       90,
			TargetPort: intstr.IntOrString{IntVal: 90},
			Protocol:   apiv1.ProtocolTCP,
		},
	}
	if !reflect.DeepEqual(result.Spec.Ports, ports) {
		t.Errorf("Wrong service ports: '%v'", result.Spec.Ports)
	}
	if result.Spec.Type != apiv1.ServiceTypeClusterIP {
		t.Errorf("Wrong service type: '%s'", result.Spec.Type)
	}
	selector := map[string]string{
		okLabels.StackNameLabel:        "stackName",
		okLabels.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Spec.Selector, selector) {
		t.Errorf("Wrong spec.selector: '%s'", result.Spec.Selector)
	}

	svc := s.Services["svcName"]
	svc.Public = true
	s.Services["svcName"] = svc
	result = translateService("svcName", s)
	annotations[okLabels.OktetoAutoIngressAnnotation] = "true"
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong service annotations: '%s'", result.Annotations)
	}
	if result.Spec.Type != apiv1.ServiceTypeLoadBalancer {
		t.Errorf("Wrong service type: '%s'", result.Spec.Type)
	}
}
