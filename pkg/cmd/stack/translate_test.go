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
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const modifiedHostName = "modified.test.hostname"

type fakeDivert struct {
	called bool
}

func (f *fakeDivert) UpdatePod(spec apiv1.PodSpec) apiv1.PodSpec {
	f.called = true
	spec.Hostname = modifiedHostName
	return spec
}

func Test_translateConfigMap(t *testing.T) {
	s := &model.Stack{
		Manifest: []byte("manifest"),
		Name:     "stack Name",
		Services: map[string]*model.Service{
			"svcName": {
				Image: "image",
			},
		},
	}
	result := translateConfigMap(s)
	if result.Name != "okteto-stack-name" {
		t.Errorf("Wrong configmap name: '%s'", result.Name)
	}
	if result.Labels[model.StackLabel] != "true" {
		t.Errorf("Wrong labels: '%s'", result.Labels)
	}
	if result.Data[NameField] != "stack Name" {
		t.Errorf("Wrong data.name: '%s'", result.Data[NameField])
	}
	if result.Data[YamlField] != base64.StdEncoding.EncodeToString(s.Manifest) {
		t.Errorf("Wrong data.yaml: '%s'", result.Data[YamlField])
	}
}

func Test_translateDeployment(t *testing.T) {
	divert := &fakeDivert{}
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				NodeSelector: model.Selector{
					"node1": "value1",
					"node2": "value2",
				},
				Image:           "image",
				Replicas:        3,
				StopGracePeriod: 20,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []env.Var{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports: []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
			},
		},
	}
	result := translateDeployment("svcName", s, divert)
	if result.Name != "svcName" {
		t.Errorf("Wrong deployment name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackname",
		model.StackServiceNameLabel: "svcName",
		model.DeployedByLabel:       "stackname",
	}
	assert.Equal(t, result.Labels, labels)
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	assert.Equal(t, result.Annotations, annotations)
	nodeSelector := map[string]string{
		"node1": "value1",
		"node2": "value2",
	}
	assert.Equal(t, result.Spec.Template.Spec.NodeSelector, nodeSelector)

	if *result.Spec.Replicas != 3 {
		t.Errorf("Wrong deployment spec.replicas: '%d'", *result.Spec.Replicas)
	}
	selector := map[string]string{
		model.StackNameLabel:        "stackname",
		model.StackServiceNameLabel: "svcName",
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

	require.Equal(t, modifiedHostName, result.Spec.Template.Spec.Hostname)
	require.True(t, divert.called)

}

func Test_translateStatefulSet(t *testing.T) {
	divert := &fakeDivert{}
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				NodeSelector: model.Selector{
					"node1": "value1",
					"node2": "value2",
				},
				Image:           "image",
				Replicas:        3,
				StopGracePeriod: 20,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []env.Var{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:   []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
				CapAdd:  []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop: []apiv1.Capability{apiv1.Capability("CAP_DROP")},

				Volumes: []build.VolumeMounts{{RemotePath: "/volume1"}, {RemotePath: "/volume2"}},
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("100m")},
						Memory: model.Quantity{Value: resource.MustParse("1Gi")},
					},
					Requests: model.ServiceResources{
						Storage: model.StorageResource{
							Size:  model.Quantity{Value: resource.MustParse("20Gi")},
							Class: "class-name",
						},
					},
				},
			},
		},
	}
	result := translateStatefulSet("svcName", s, divert)
	if result.Name != "svcName" {
		t.Errorf("Wrong statefulset name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackname",
		model.StackServiceNameLabel: "svcName",
		model.DeployedByLabel:       "stackname",
	}
	assert.Equal(t, labels, result.Labels)
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	assert.Equal(t, result.Annotations, annotations)
	nodeSelector := map[string]string{
		"node1": "value1",
		"node2": "value2",
	}
	assert.Equal(t, result.Spec.Template.Spec.NodeSelector, nodeSelector)

	if *result.Spec.Replicas != 3 {
		t.Errorf("Wrong statefulset spec.replicas: '%d'", *result.Spec.Replicas)
	}
	selector := map[string]string{
		model.StackNameLabel:        "stackname",
		model.StackServiceNameLabel: "svcName",
	}
	if !reflect.DeepEqual(result.Spec.Selector.MatchLabels, selector) {
		t.Errorf("Wrong spec.selector: '%s'", result.Spec.Selector.MatchLabels)
	}
	assert.Equal(t, labels, result.Spec.Template.Labels)
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong statefulset spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	initContainer := apiv1.Container{
		Name:    fmt.Sprintf("init-%s", "svcName"),
		Image:   "busybox",
		Command: []string{"sh", "-c", "chmod 777 /data"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/data",
				Name:      pvcName,
			},
		},
	}
	assert.Equal(t, initContainer, result.Spec.Template.Spec.InitContainers[0])
	initVolumeContainer := apiv1.Container{
		Name:            fmt.Sprintf("init-volume-%s", "svcName"),
		Image:           "image",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         []string{"sh", "-c", "echo initializing volume... && (cp -Rv /volume1/. /init-volume-0 || true) && (cp -Rv /volume2/. /init-volume-1 || true)"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/init-volume-0",
				Name:      pvcName,
				SubPath:   "data-0",
			},
			{
				MountPath: "/init-volume-1",
				Name:      pvcName,
				SubPath:   "data-1",
			},
		},
	}
	assert.Equal(t, initVolumeContainer, result.Spec.Template.Spec.InitContainers[1])

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
	assert.Equal(t, volumeMounts, c.VolumeMounts)

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
		StorageClassName: pointer.String("class-name"),
	}
	if !reflect.DeepEqual(vct.Spec, volumeClaimTemplateSpec) {
		t.Errorf("Wrong statefulset volume claim template: '%v'", vct.Spec)
	}

	require.Equal(t, modifiedHostName, result.Spec.Template.Spec.Hostname)
	require.True(t, divert.called)

}

func Test_translateJobWithoutVolumes(t *testing.T) {
	divert := &fakeDivert{}
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				NodeSelector: model.Selector{
					"node1": "value1",
					"node2": "value2",
				},
				Image:           "image",
				StopGracePeriod: 20,
				Replicas:        3,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []env.Var{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:         []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
				CapAdd:        []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop:       []apiv1.Capability{apiv1.Capability("CAP_DROP")},
				RestartPolicy: apiv1.RestartPolicyNever,
				BackOffLimit:  5,
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("100m")},
						Memory: model.Quantity{Value: resource.MustParse("1Gi")},
					},
					Requests: model.ServiceResources{
						Storage: model.StorageResource{
							Size:  model.Quantity{Value: resource.MustParse("20Gi")},
							Class: "class-name",
						},
					},
				},
			},
		},
	}
	result := translateJob("svcName", s, divert)
	if result.Name != "svcName" {
		t.Errorf("Wrong job name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackname",
		model.StackServiceNameLabel: "svcName",
		model.DeployedByLabel:       "stackname",
	}
	assert.Equal(t, labels, result.Labels)
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	assert.Equal(t, result.Annotations, annotations)
	nodeSelector := map[string]string{
		"node1": "value1",
		"node2": "value2",
	}
	assert.Equal(t, result.Spec.Template.Spec.NodeSelector, nodeSelector)
	if *result.Spec.Completions != 3 {
		t.Errorf("Wrong job spec.completions: '%d'", *result.Spec.Completions)
	}
	if *result.Spec.Parallelism != 1 {
		t.Errorf("Wrong job spec.parallelism: '%d'", *result.Spec.Parallelism)
	}
	if *result.Spec.BackoffLimit != 5 {
		t.Errorf("Wrong job spec.max_attempts: '%d'", *result.Spec.BackoffLimit)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong job spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	if len(result.Spec.Template.Spec.InitContainers) > 0 {
		t.Errorf("Wrong job spec.template.spec.initContainers: '%d'", len(result.Spec.Template.Spec.InitContainers))
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong job container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong job container.image: '%s'", c.Image)
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
		t.Errorf("Wrong job container.security_context: '%v'", c.SecurityContext)
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
	if len(c.VolumeMounts) > 0 {
		t.Errorf("Wrong c.VolumeMounts: '%d'", len(c.VolumeMounts))
	}

	require.Equal(t, modifiedHostName, result.Spec.Template.Spec.Hostname)
	require.True(t, divert.called)
}

func Test_translateJobWithVolumes(t *testing.T) {
	divert := &fakeDivert{}
	s := &model.Stack{
		Name: "stackName",
		Services: map[string]*model.Service{
			"svcName": {
				Labels: model.Labels{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: model.Annotations{
					"annotation1": "value1",
					"annotation2": "value2",
				},
				NodeSelector: model.Selector{
					"node1": "value1",
					"node2": "value2",
				},
				Image:           "image",
				StopGracePeriod: 20,
				Replicas:        3,
				Entrypoint:      model.Entrypoint{Values: []string{"command1", "command2"}},
				Command:         model.Command{Values: []string{"args1", "args2"}},
				Environment: []env.Var{
					{
						Name:  "env1",
						Value: "value1",
					},
					{
						Name:  "env2",
						Value: "value2",
					},
				},
				Ports:         []model.Port{{ContainerPort: 80}, {ContainerPort: 90}},
				CapAdd:        []apiv1.Capability{apiv1.Capability("CAP_ADD")},
				CapDrop:       []apiv1.Capability{apiv1.Capability("CAP_DROP")},
				RestartPolicy: apiv1.RestartPolicyNever,
				BackOffLimit:  5,
				Volumes:       []build.VolumeMounts{{RemotePath: "/volume1"}, {RemotePath: "/volume2"}},
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("100m")},
						Memory: model.Quantity{Value: resource.MustParse("1Gi")},
					},
					Requests: model.ServiceResources{
						Storage: model.StorageResource{
							Size:  model.Quantity{Value: resource.MustParse("20Gi")},
							Class: "class-name",
						},
					},
				},
			},
		},
	}
	result := translateJob("svcName", s, divert)
	if result.Name != "svcName" {
		t.Errorf("Wrong job name: '%s'", result.Name)
	}
	labels := map[string]string{
		"label1":                    "value1",
		"label2":                    "value2",
		model.StackNameLabel:        "stackname",
		model.StackServiceNameLabel: "svcName",
		model.DeployedByLabel:       "stackname",
	}
	assert.Equal(t, labels, result.Labels)
	annotations := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}
	assert.Equal(t, result.Annotations, annotations)
	nodeSelector := map[string]string{
		"node1": "value1",
		"node2": "value2",
	}
	assert.Equal(t, result.Spec.Template.Spec.NodeSelector, nodeSelector)
	if !reflect.DeepEqual(result.Annotations, annotations) {
		t.Errorf("Wrong job annotations: '%s'", result.Annotations)
	}
	if *result.Spec.Completions != 3 {
		t.Errorf("Wrong job spec.completions: '%d'", *result.Spec.Completions)
	}
	if *result.Spec.Parallelism != 1 {
		t.Errorf("Wrong job spec.parallelism: '%d'", *result.Spec.Parallelism)
	}
	if *result.Spec.BackoffLimit != 5 {
		t.Errorf("Wrong job spec.max_attempts: '%d'", *result.Spec.BackoffLimit)
	}
	if !reflect.DeepEqual(result.Spec.Template.Labels, labels) {
		t.Errorf("Wrong spec.template.labels: '%s'", result.Spec.Template.Labels)
	}
	if !reflect.DeepEqual(result.Spec.Template.Annotations, annotations) {
		t.Errorf("Wrong spec.template.annotations: '%s'", result.Spec.Template.Annotations)
	}
	if *result.Spec.Template.Spec.TerminationGracePeriodSeconds != 20 {
		t.Errorf("Wrong job spec.template.spec.termination_grade_period_seconds: '%d'", *result.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
	initContainer := apiv1.Container{
		Name:    fmt.Sprintf("init-%s", "svcName"),
		Image:   "busybox",
		Command: []string{"sh", "-c", "chmod 777 /data"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/data",
				Name:      pvcName,
			},
		},
	}
	if !reflect.DeepEqual(result.Spec.Template.Spec.InitContainers[0], initContainer) {
		t.Errorf("Wrong job init container: '%v' but expected '%v'", result.Spec.Template.Spec.InitContainers[0], initContainer)
	}
	initVolumeContainer := apiv1.Container{
		Name:            fmt.Sprintf("init-volume-%s", "svcName"),
		Image:           "image",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         []string{"sh", "-c", "echo initializing volume... && (cp -Rv /volume1/. /init-volume-0 || true) && (cp -Rv /volume2/. /init-volume-1 || true)"},
		VolumeMounts: []apiv1.VolumeMount{
			{
				MountPath: "/init-volume-0",
				Name:      pvcName,
				SubPath:   "data-0",
			},
			{
				MountPath: "/init-volume-1",
				Name:      pvcName,
				SubPath:   "data-1",
			},
		},
	}
	if !reflect.DeepEqual(result.Spec.Template.Spec.InitContainers[1], initVolumeContainer) {
		t.Errorf("Wrong job init container: '%v' but expected '%v'", result.Spec.Template.Spec.InitContainers[1], initVolumeContainer)
	}
	c := result.Spec.Template.Spec.Containers[0]
	if c.Name != "svcName" {
		t.Errorf("Wrong job container.name: '%s'", c.Name)
	}
	if c.Image != "image" {
		t.Errorf("Wrong job container.image: '%s'", c.Image)
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
		t.Errorf("Wrong job container.security_context: '%v'", c.SecurityContext)
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

	require.Equal(t, modifiedHostName, result.Spec.Template.Spec.Hostname)
	require.True(t, divert.called)
}

func Test_translateService(t *testing.T) {

	var tests = []struct {
		stack    *model.Stack
		expected *apiv1.Service
		name     string
	}{
		{
			name: "translate svc no public endpoints",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1": "value1",
							"annotation2": "value2",
						},
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							},
						},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
						model.DeployedByLabel:       "stackname",
					},
					Annotations: map[string]string{
						"annotation1": "value1",
						"annotation2": "value2",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc public endpoints",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},

						Public: true,
						Annotations: model.Annotations{
							"annotation1":                     "value1",
							"annotation2":                     "value2",
							model.OktetoAutoIngressAnnotation: "true",
						},
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							},
						},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
						model.DeployedByLabel:       "stackname",
					},
					Annotations: map[string]string{
						"annotation1":                     "value1",
						"annotation2":                     "value2",
						model.OktetoAutoIngressAnnotation: "true",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc private endpoints",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1":                     "value1",
							"annotation2":                     "value2",
							model.OktetoAutoIngressAnnotation: "private",
						},
						Public: true,
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							}},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
						model.DeployedByLabel:       "stackname",
					},
					Annotations: map[string]string{
						"annotation1":                     "value1",
						"annotation2":                     "value2",
						model.OktetoAutoIngressAnnotation: "private",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc private endpoints by private annotation",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1":                    "value1",
							"annotation2":                    "value2",
							model.OktetoPrivateSvcAnnotation: "true",
						},
						Public: true,
						Ports: []model.Port{
							{
								HostPort:      82,
								ContainerPort: 80,
								Protocol:      apiv1.ProtocolTCP,
							},
							{
								ContainerPort: 90,
								Protocol:      apiv1.ProtocolTCP,
							}},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
						model.DeployedByLabel:       "stackname",
					},
					Annotations: map[string]string{
						"annotation1":                    "value1",
						"annotation2":                    "value2",
						model.OktetoPrivateSvcAnnotation: "true",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-80-80-tcp",
							Port:       80,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-82-80-tcp",
							Port:       82,
							TargetPort: intstr.IntOrString{IntVal: 80},
							Protocol:   apiv1.ProtocolTCP,
						},
						{
							Name:       "p-90-90-tcp",
							Port:       90,
							TargetPort: intstr.IntOrString{IntVal: 90},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
		{
			name: "translate svc private endpoints by private annotation",
			stack: &model.Stack{
				Name: "stackName",
				Services: map[string]*model.Service{
					"svcName": {
						Labels: model.Labels{
							"label1": "value1",
							"label2": "value2",
						},
						Annotations: model.Annotations{
							"annotation1": "value1",
							"annotation2": "value2",
						},
						Ports: []model.Port{
							{
								HostPort:      6379,
								ContainerPort: 6379,
								Protocol:      apiv1.ProtocolTCP,
							},
						},
					},
				},
			},
			expected: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svcName",
					Labels: map[string]string{
						"label1":                    "value1",
						"label2":                    "value2",
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
						model.DeployedByLabel:       "stackname",
					},
					Annotations: map[string]string{
						"annotation1": "value1",
						"annotation2": "value2",
					},
				},
				Spec: apiv1.ServiceSpec{
					Type: apiv1.ServiceTypeClusterIP,
					Selector: map[string]string{
						model.StackNameLabel:        "stackname",
						model.StackServiceNameLabel: "svcName",
					},
					Ports: []apiv1.ServicePort{
						{
							Name:       "p-6379-6379-tcp",
							Port:       6379,
							TargetPort: intstr.IntOrString{IntVal: 6379},
							Protocol:   apiv1.ProtocolTCP,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateService("svcName", tt.stack)
			assert.Equal(t, tt.expected, result)
		})
	}

}

func Test_translateSvcProbe(t *testing.T) {
	tests := []struct {
		expected healthcheckProbes
		svc      *model.Service
		name     string
	}{
		{
			name: "nil healthcheck",
			svc: &model.Service{
				Healtcheck: nil,
			},
			expected: healthcheckProbes{
				readiness: nil,
				liveness:  nil,
			},
		},
		{
			name: "healthcheck http",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					HTTP: &model.HTTPHealtcheck{
						Path: "/",
						Port: 8080,
					},
					Readiness: true,
				},
			},
			expected: healthcheckProbes{
				readiness: &apiv1.Probe{
					ProbeHandler: apiv1.ProbeHandler{
						HTTPGet: &apiv1.HTTPGetAction{
							Path: "/",
							Port: intstr.IntOrString{IntVal: 8080},
						},
					},
				},
				liveness: nil,
			},
		},

		{
			name: "healthcheck http with other fields both ",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					HTTP: &model.HTTPHealtcheck{
						Path: "/",
						Port: 8080,
					},
					StartPeriod: 30 * time.Second,
					Retries:     5,
					Timeout:     5 * time.Minute,
					Interval:    45 * time.Second,
					Readiness:   true,
					Liveness:    true,
				},
			},
			expected: healthcheckProbes{
				readiness: &apiv1.Probe{
					ProbeHandler: apiv1.ProbeHandler{
						HTTPGet: &apiv1.HTTPGetAction{
							Path: "/",
							Port: intstr.IntOrString{IntVal: 8080},
						},
					},
					InitialDelaySeconds: 30,
					FailureThreshold:    5,
					TimeoutSeconds:      300,
					PeriodSeconds:       45,
				},
				liveness: &apiv1.Probe{
					ProbeHandler: apiv1.ProbeHandler{
						HTTPGet: &apiv1.HTTPGetAction{
							Path: "/",
							Port: intstr.IntOrString{IntVal: 8080},
						},
					},
					InitialDelaySeconds: 30,
					FailureThreshold:    5,
					TimeoutSeconds:      300,
					PeriodSeconds:       45,
				},
			},
		},
		{
			name: "healthcheck exec only readiness",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					Test: model.HealtcheckTest{
						"curl", "db-service:8080/readiness",
					},
					Readiness: true,
				},
			},
			expected: healthcheckProbes{
				readiness: &apiv1.Probe{
					ProbeHandler: apiv1.ProbeHandler{
						Exec: &apiv1.ExecAction{
							Command: []string{"curl", "db-service:8080/readiness"},
						},
					},
				},
				liveness: nil,
			},
		},
		{
			name: "healthcheck exec with others fields only liveness",
			svc: &model.Service{
				Healtcheck: &model.HealthCheck{
					Test: model.HealtcheckTest{
						"curl", "db-service:8080/readiness",
					},
					StartPeriod: 30 * time.Second,
					Retries:     5,
					Timeout:     5 * time.Minute,
					Interval:    45 * time.Second,
					Liveness:    true,
				},
			},
			expected: healthcheckProbes{
				liveness: &apiv1.Probe{
					ProbeHandler: apiv1.ProbeHandler{
						Exec: &apiv1.ExecAction{
							Command: []string{"curl", "db-service:8080/readiness"},
						},
					},
					InitialDelaySeconds: 30,
					FailureThreshold:    5,
					TimeoutSeconds:      300,
					PeriodSeconds:       45,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := getSvcHealthProbe(tt.svc)
			assert.Equal(t, tt.expected, probe)
		})
	}
}

func Test_translateServiceEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		svc      *model.Service
		expected []apiv1.EnvVar
	}{
		{
			name: "none",
			svc: &model.Service{
				Environment: env.Environment{},
			},
			expected: []apiv1.EnvVar{},
		},
		{
			name: "empty value",
			svc: &model.Service{
				Environment: env.Environment{
					env.Var{
						Name: "DEBUG",
					},
				},
			},
			expected: []apiv1.EnvVar{
				{
					Name: "DEBUG",
				},
			},
		},
		{
			name: "empty name",
			svc: &model.Service{
				Environment: env.Environment{
					env.Var{
						Value: "DEBUG",
					},
				},
			},
			expected: []apiv1.EnvVar{},
		},
		{
			name: "ok env var",
			svc: &model.Service{
				Environment: env.Environment{
					env.Var{
						Name:  "DEBUG",
						Value: "true",
					},
				},
			},
			expected: []apiv1.EnvVar{
				{
					Name:  "DEBUG",
					Value: "true",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envs := translateServiceEnvironment(tt.svc)
			if !reflect.DeepEqual(tt.expected, envs) {
				t.Fatal("Wrong translation")
			}
		})
	}
}

func Test_translateResources(t *testing.T) {
	tests := []struct {
		svc       *model.Service
		name      string
		resources apiv1.ResourceRequirements
	}{
		{
			name: "svc not defined",
			svc: &model.Service{
				Resources: nil,
			},
			resources: apiv1.ResourceRequirements{},
		},
		{
			name: "svc Limits CPU defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU: model.Quantity{Value: resource.MustParse("2")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Limits: apiv1.ResourceList{
					apiv1.ResourceCPU: resource.MustParse("2"),
				},
			},
		},
		{
			name: "svc Limits Memory defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						Memory: model.Quantity{Value: resource.MustParse("5Gi")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Limits: apiv1.ResourceList{
					apiv1.ResourceMemory: resource.MustParse("5Gi"),
				},
			},
		},
		{
			name: "svc Limits ALL defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("2")},
						Memory: model.Quantity{Value: resource.MustParse("5Gi")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Limits: apiv1.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("2"),
					apiv1.ResourceMemory: resource.MustParse("5Gi"),
				},
			},
		},

		{
			name: "svc Requests CPU defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Requests: model.ServiceResources{
						CPU: model.Quantity{Value: resource.MustParse("11m")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU: resource.MustParse("11m"),
				},
			},
		},
		{
			name: "svc Requests Memory defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Requests: model.ServiceResources{
						Memory: model.Quantity{Value: resource.MustParse("60Mi")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceMemory: resource.MustParse("60Mi"),
				},
			},
		},
		{
			name: "svc Requests ALL defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Requests: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("11m")},
						Memory: model.Quantity{Value: resource.MustParse("60Mi")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("11m"),
					apiv1.ResourceMemory: resource.MustParse("60Mi"),
				},
			},
		},
		{
			name: "svc ALL defined",
			svc: &model.Service{
				Resources: &model.StackResources{
					Limits: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("2")},
						Memory: model.Quantity{Value: resource.MustParse("5Gi")},
					},
					Requests: model.ServiceResources{
						CPU:    model.Quantity{Value: resource.MustParse("11m")},
						Memory: model.Quantity{Value: resource.MustParse("60Mi")},
					},
				},
			},
			resources: apiv1.ResourceRequirements{
				Limits: apiv1.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("2"),
					apiv1.ResourceMemory: resource.MustParse("5Gi"),
				},
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU:    resource.MustParse("11m"),
					apiv1.ResourceMemory: resource.MustParse("60Mi"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := translateResources(tt.svc)
			if !reflect.DeepEqual(tt.resources, res) {
				t.Fatalf("Wrong translation expected %v, got %v", tt.resources, res)
			}
		})
	}
}

func Test_translateAffinity(t *testing.T) {
	tests := []struct {
		svc                   *model.Service
		affinity              *apiv1.Affinity
		name                  string
		disableVolumeAffinity bool
	}{
		{
			name: "none",
			svc: &model.Service{
				Environment: env.Environment{},
			},
			affinity: nil,
		},
		{
			name: "only volume mounts",
			svc: &model.Service{
				VolumeMounts: []build.VolumeMounts{
					{
						LocalPath:  "",
						RemotePath: "/var",
					},
				},
			},
			affinity: nil,
		},
		{
			name: "one volume",
			svc: &model.Service{
				Volumes: []build.VolumeMounts{
					{
						LocalPath:  "test",
						RemotePath: "/var",
					},
				},
			},
			affinity: &apiv1.Affinity{
				PodAffinity: &apiv1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple volumes",
			svc: &model.Service{
				Volumes: []build.VolumeMounts{
					{
						LocalPath:  "test-1",
						RemotePath: "/var",
					},
					{
						LocalPath:  "test-2",
						RemotePath: "/var",
					},
					{
						LocalPath:  "test-3",
						RemotePath: "/var",
					},
				},
			},
			affinity: &apiv1.Affinity{
				PodAffinity: &apiv1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test-1", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test-2", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      fmt.Sprintf("%s-test-3", model.StackVolumeNameLabel),
										Operator: metav1.LabelSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple volumes with volume affinity disabled",
			svc: &model.Service{
				Volumes: []build.VolumeMounts{
					{
						LocalPath:  "test-1",
						RemotePath: "/var",
					},
					{
						LocalPath:  "test-2",
						RemotePath: "/var",
					},
					{
						LocalPath:  "test-3",
						RemotePath: "/var",
					},
				},
			},
			disableVolumeAffinity: true,
			affinity:              nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.disableVolumeAffinity {
				t.Setenv(oktetoComposeVolumeAffinityEnabledEnvVar, "false")
			}
			aff := translateAffinity(tt.svc)
			assert.Equal(t, tt.affinity, aff)
		})
	}
}

func TestGetSvcPublicPorts(t *testing.T) {
	tests := []struct {
		stack          *model.Stack
		name           string
		svcName        string
		expectedLength int
	}{
		{
			name:    "one public port",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Ports: []model.Port{
							{
								HostPort:      80,
								ContainerPort: 80,
							},
						},
					},
				},
			},
			expectedLength: 1,
		},
		{
			name:    "one private port",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Ports: []model.Port{
							{
								ContainerPort: 80,
							},
						},
					},
				},
			},
			expectedLength: 0,
		},
		{
			name:    "one public port with public field",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Public: true,
						Ports: []model.Port{
							{
								ContainerPort: 80,
								HostPort:      80,
							},
						},
					},
				},
			},
			expectedLength: 1,
		},
		{
			name:    "one public port",
			svcName: "test",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test": {
						Public: true,
						Ports: []model.Port{
							{
								HostPort:      80,
								ContainerPort: 80,
							},
						},
					},
				},
			},
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := getSvcPublicPorts(tt.svcName, tt.stack)
			assert.Len(t, ports, tt.expectedLength)
		})
	}
}

func TestGetDeploymentStrategy(t *testing.T) {
	tests := []struct {
		expected appsv1.DeploymentStrategy
		svc      *model.Service
		envs     map[string]string
		name     string
	}{
		{
			name: "default",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
		{
			name: "annotation ok - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(rollingUpdateStrategy),
				},
			},
			envs: map[string]string{},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
		{
			name: "annotation ok with different env var - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(rollingUpdateStrategy),
				},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(recreateUpdateStrategy),
			},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
		{
			name: "annotation not ok - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(onDeleteUpdateStrategy),
				},
			},
			envs: map[string]string{},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
		{
			name: "annotation ok - recreate",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(recreateUpdateStrategy),
				},
			},
			envs: map[string]string{},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
		{
			name: "env var ok - recreate",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(recreateUpdateStrategy),
			},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
		{
			name: "env var ok - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(rollingUpdateStrategy),
			},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
		{
			name: "env var not ok",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(onDeleteUpdateStrategy),
			},
			expected: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			result := getDeploymentUpdateStrategy(tt.svc)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStrategyStrategy(t *testing.T) {
	tests := []struct {
		expected appsv1.StatefulSetUpdateStrategy
		svc      *model.Service
		envs     map[string]string
		name     string
	}{
		{
			name: "default",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		{
			name: "annotation ok - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(rollingUpdateStrategy),
				},
			},
			envs: map[string]string{},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		{
			name: "annotation ok with different env var - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(rollingUpdateStrategy),
				},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(recreateUpdateStrategy),
			},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		{
			name: "annotation not ok - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(recreateUpdateStrategy),
				},
			},
			envs: map[string]string{},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		{
			name: "annotation ok - on-delete",
			svc: &model.Service{
				Annotations: model.Annotations{
					model.OktetoComposeUpdateStrategyAnnotation: string(onDeleteUpdateStrategy),
				},
			},
			envs: map[string]string{},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
		},
		{
			name: "env var ok - on-delete",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(onDeleteUpdateStrategy),
			},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
		},
		{
			name: "env var ok - rolling",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(rollingUpdateStrategy),
			},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		{
			name: "env var not ok",
			svc: &model.Service{
				Annotations: model.Annotations{},
			},
			envs: map[string]string{
				model.OktetoComposeUpdateStrategyEnvVar: string(recreateUpdateStrategy),
			},
			expected: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			result := getStatefulsetUpdateStrategy(tt.svc)
			assert.Equal(t, tt.expected, result)
		})
	}
}
