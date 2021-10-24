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

package apps

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/model"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

var (
	mode444 int32 = 0444
	mode420 int32 = 420
)

func Test_translateWithVolumes(t *testing.T) {
	file, err := os.CreateTemp("/tmp", "okteto-secret-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	var runAsUser int64 = 100
	var runAsGroup int64 = 101
	var fsGroup int64 = 102
	manifest := []byte(fmt.Sprintf(`name: web
namespace: n
container: dev
image: web:latest
annotations:
  key1: value1
command: ["./run_web.sh"]
metadata:
  labels:
    app: web
workdir: /app
securityContext:
  runAsUser: 100
  runAsGroup: 101
  fsGroup: 102
serviceAccount: sa
sync:
  - .:/app
  - sub:/path
volumes:
  - /go/pkg/
  - /root/.cache/go-build
tolerations:
  - key: nvidia/gpu
    operator: Exists
nodeSelector:
  disktype: ssd
affinity:
  podAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: role
          operator: In
          values:
          - web-server
      topologyKey: kubernetes.io/hostname
secrets:
  - %s:/remote
resources:
  limits:
    cpu: 2
    memory: 1Gi
    nvidia.com/gpu: 1
    amd.com/gpu: 1
services:
  - name: worker
    container: dev
    image: worker:latest
    command: ["./run_worker.sh"]
    annotations:
      key2: value2
    sync:
       - worker:/src`, file.Name()))

	dev1, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	d1 := deployments.Sandbox(dev1)
	d1.UID = types.UID("deploy1")
	delete(d1.Annotations, model.OktetoAutoCreateAnnotation)
	d1.Spec.Replicas = pointer.Int32Ptr(2)
	d1.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
	}
	rule1 := dev1.ToTranslationRule(dev1, false)
	tr1 := &Translation{
		MainDev: dev1,
		Dev:     dev1,
		App:     NewDeploymentApp(d1),
		Rules:   []*model.TranslationRule{rule1},
	}
	if err := tr1.translate(); err != nil {
		t.Fatal(err)
	}
	dDevPod1OK := apiv1.PodSpec{
		NodeSelector: map[string]string{
			"disktype": "ssd",
		},
		Affinity: &apiv1.Affinity{
			PodAffinity: &apiv1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "role",
									Operator: "In",
									Values: []string{
										"web-server",
									},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
		Tolerations: []apiv1.Toleration{
			{
				Key:      "nvidia/gpu",
				Operator: apiv1.TolerationOpExists,
			},
		},
		SecurityContext: &apiv1.PodSecurityContext{
			FSGroup: &fsGroup,
		},
		ServiceAccountName:            "sa",
		TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
		Volumes: []apiv1.Volume{
			{
				Name: oktetoSyncSecretVolume,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "okteto-web",
						Items: []apiv1.KeyToPath{
							{
								Key:  "config.xml",
								Path: "config.xml",
								Mode: &mode444,
							},
							{
								Key:  "cert.pem",
								Path: "cert.pem",
								Mode: &mode444,
							},
							{
								Key:  "key.pem",
								Path: "key.pem",
								Mode: &mode444,
							},
						},
					},
				},
			},
			{
				Name: dev1.GetVolumeName(),
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: dev1.GetVolumeName(),
						ReadOnly:  false,
					},
				},
			},
			{
				Name: oktetoDevSecretVolume,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "okteto-web",
						Items: []apiv1.KeyToPath{
							{
								Key:  "dev-secret-remote",
								Path: "remote",
								Mode: &mode420,
							},
						},
					},
				},
			},
			{
				Name: OktetoBinName,
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []apiv1.Container{
			{
				Name:            OktetoBinName,
				Image:           model.OktetoBinImageTag,
				ImagePullPolicy: apiv1.PullIfNotPresent,
				Command:         []string{"sh", "-c", "cp /usr/local/bin/* /okteto/bin"},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  &runAsUser,
					RunAsGroup: &runAsGroup,
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      OktetoBinName,
						MountPath: "/okteto/bin",
					},
				},
			},
			{
				Name:            OktetoInitVolumeContainerName,
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullIfNotPresent,
				Command:         []string{"sh", "-cx", "echo initializing && ( [ \"$(ls -A /init-volume/1)\" ] || cp -R /go/pkg/. /init-volume/1 || true) && ( [ \"$(ls -A /init-volume/2)\" ] || cp -R /root/.cache/go-build/. /init-volume/2 || true) && ( [ \"$(ls -A /init-volume/3)\" ] || cp -R /app/. /init-volume/3 || true) && ( [ \"$(ls -A /init-volume/4)\" ] || cp -R /path/. /init-volume/4 || true)"},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  &runAsUser,
					RunAsGroup: &runAsGroup,
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/1",
						SubPath:   path.Join(model.DataSubPath, "go/pkg"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/2",
						SubPath:   path.Join(model.DataSubPath, "root/.cache/go-build"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/3",
						SubPath:   model.SourceCodeSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/4",
						SubPath:   path.Join(model.SourceCodeSubPath, "sub"),
					},
				},
			},
		},
		Containers: []apiv1.Container{
			{
				Name:            "dev",
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullAlways,
				Command:         []string{"/var/okteto/bin/start.sh"},
				Args:            []string{"-r", "-s", "remote:/remote"},
				WorkingDir:      "/app",
				Env: []apiv1.EnvVar{
					{
						Name:  "OKTETO_NAMESPACE",
						Value: "n",
					},
					{
						Name:  "OKTETO_NAME",
						Value: "web",
					},
				},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  &runAsUser,
					RunAsGroup: &runAsGroup,
				},
				Resources: apiv1.ResourceRequirements{
					Limits: apiv1.ResourceList{
						"cpu":            resource.MustParse("2"),
						"memory":         resource.MustParse("1Gi"),
						"nvidia.com/gpu": resource.MustParse("1"),
						"amd.com/gpu":    resource.MustParse("1"),
					},
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/var/syncthing",
						SubPath:   model.SyncthingSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: model.RemoteMountPath,
						SubPath:   model.RemoteSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/go/pkg/",
						SubPath:   path.Join(model.DataSubPath, "go/pkg"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/root/.cache/go-build",
						SubPath:   path.Join(model.DataSubPath, "root/.cache/go-build"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/app",
						SubPath:   model.SourceCodeSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/path",
						SubPath:   path.Join(model.SourceCodeSubPath, "sub"),
					},
					{
						Name:      oktetoSyncSecretVolume,
						ReadOnly:  false,
						MountPath: "/var/syncthing/secret/",
					},
					{
						Name:      oktetoDevSecretVolume,
						ReadOnly:  false,
						MountPath: "/var/okteto/secret/",
					},
					{
						Name:      OktetoBinName,
						ReadOnly:  false,
						MountPath: "/var/okteto/bin",
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
			},
		},
	}

	//checking d1 state
	d1App := tr1.App.(*DeploymentApp)
	if !reflect.DeepEqual(d1App.d.Spec.Strategy, appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}) {
		t.Fatalf("d1 wrong strategy %v", d1App.d.Spec.Strategy)
	}

	d1Orig := deployments.Sandbox(dev1)
	if tr1.App.Replicas() != 0 {
		t.Fatalf("d1 is running %d replicas", tr1.App.Replicas())
	}
	expectedLabels := map[string]string{model.DevLabel: "true"}
	if !reflect.DeepEqual(tr1.App.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong d1 labels: '%v'", tr1.App.ObjectMeta().Labels)
	}

	if !reflect.DeepEqual(tr1.App.TemplateObjectMeta().Labels, d1Orig.Spec.Template.Labels) {
		t.Fatalf("Wrong d1 pod labels: '%v'", tr1.App.TemplateObjectMeta().Labels)

	}
	expectedAnnotations := map[string]string{model.AppReplicasAnnotation: "2"}
	if !reflect.DeepEqual(tr1.App.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong d1 annotations: '%v'", tr1.App.ObjectMeta().Annotations)
	}
	if !reflect.DeepEqual(tr1.App.TemplateObjectMeta().Annotations, d1.Spec.Template.Annotations) {
		t.Fatalf("Wrong d1 pod annotations: '%v'", tr1.App.TemplateObjectMeta().Annotations)
	}
	marshalledD1, _ := yaml.Marshal(tr1.App.PodSpec())
	marshalledD1Orig, _ := yaml.Marshal(d1Orig.Spec.Template.Spec)
	if string(marshalledD1) != string(marshalledD1Orig) {
		t.Fatalf("Wrong sfs1 generation.\nActual %+v, \nExpected %+v", string(marshalledD1), string(marshalledD1Orig))
	}

	//checking dev d1 state
	devD1App := tr1.DevApp.(*DeploymentApp)
	if !reflect.DeepEqual(devD1App.d.Spec.Strategy, appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}) {
		t.Fatalf("dev d1 wrong strategy %v", d1App.d.Spec.Strategy)
	}
	if tr1.DevApp.Replicas() != 1 {
		t.Fatalf("dev d1 is running %d replicas", tr1.DevApp.Replicas())
	}
	expectedLabels = map[string]string{model.DevCloneLabel: "deploy1", "app": "web"}
	for k, v := range expectedLabels {
		if devValue, ok := tr1.DevApp.ObjectMeta().Labels[k]; ok && devValue != v {
			t.Fatalf("Wrong dev d1 labels: '%v'", tr1.DevApp.ObjectMeta().Labels)
		} else if !ok {
			t.Fatalf("Wrong dev d1 labels: '%v'", tr1.DevApp.ObjectMeta().Labels)
		}
	}

	expectedPodLabels := map[string]string{"app": "web", model.InteractiveDevLabel: dev1.Name}
	if !reflect.DeepEqual(tr1.DevApp.TemplateObjectMeta().Labels, expectedPodLabels) {
		t.Fatalf("Wrong dev d1 pod labels: '%v'", tr1.DevApp.TemplateObjectMeta().Labels)
	}
	expectedAnnotations = map[string]string{"key1": "value1"}
	if !reflect.DeepEqual(tr1.DevApp.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong dev d1 annotations: '%v'", tr1.DevApp.ObjectMeta().Annotations)
	}
	expectedPodAnnotations := map[string]string{"key1": "value1"}
	if !reflect.DeepEqual(tr1.DevApp.TemplateObjectMeta().Annotations, expectedPodAnnotations) {
		t.Fatalf("Wrong dev d1 pod annotations: '%v'", tr1.DevApp.TemplateObjectMeta().Annotations)
	}
	marshalledDevD1, _ := yaml.Marshal(tr1.DevApp.PodSpec())
	marshalledDevD1OK, _ := yaml.Marshal(dDevPod1OK)
	if string(marshalledDevD1) != string(marshalledDevD1OK) {
		t.Fatalf("Wrong dev d1 generation.\nActual %+v, \nExpected %+v", string(marshalledDevD1), string(marshalledDevD1OK))
	}

	tr1.DevModeOff()

	if _, ok := tr1.App.ObjectMeta().Labels[model.DevLabel]; ok {
		t.Fatalf("'%s' label not eliminated on 'okteto down'", model.DevLabel)
	}

	if tr1.App.Replicas() != 2 {
		t.Fatalf("d1 is running %d replicas after 'okteto down'", tr1.App.Replicas())
	}

	dev2 := dev1.Services[0]
	d2 := deployments.Sandbox(dev2)
	d2.UID = types.UID("deploy2")
	delete(d2.Annotations, model.OktetoAutoCreateAnnotation)
	d2.Spec.Replicas = pointer.Int32Ptr(3)
	d2.Namespace = dev1.Namespace

	translationRules := make(map[string]*Translation)
	ctx := context.Background()

	c := fake.NewSimpleClientset(d2)
	if err := loadServiceTranslations(ctx, dev1, false, translationRules, c); err != nil {
		t.Fatal(err)
	}
	tr2 := translationRules[dev2.Name]
	if err := tr2.translate(); err != nil {
		t.Fatal(err)
	}
	d2DevPodOK := apiv1.PodSpec{
		Affinity: &apiv1.Affinity{
			PodAffinity: &apiv1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								model.InteractiveDevLabel: "web",
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
		SecurityContext: &apiv1.PodSecurityContext{
			FSGroup: pointer.Int64Ptr(0),
		},
		ServiceAccountName:            "",
		TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
		Volumes: []apiv1.Volume{
			{
				Name: dev1.GetVolumeName(),
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: dev1.GetVolumeName(),
						ReadOnly:  false,
					},
				},
			},
		},
		Containers: []apiv1.Container{
			{
				Name:            "dev",
				Image:           "worker:latest",
				ImagePullPolicy: apiv1.PullAlways,
				Command:         []string{"./run_worker.sh"},
				Args:            []string{},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  pointer.Int64Ptr(0),
					RunAsGroup: pointer.Int64Ptr(0),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/src",
						SubPath:   path.Join(model.SourceCodeSubPath, "worker"),
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
			},
		},
	}

	//checking d2 state
	d2Orig := deployments.Sandbox(dev2)
	if tr2.App.Replicas() != 0 {
		t.Fatalf("d2 is running %d replicas", tr2.App.Replicas())
	}
	expectedLabels = map[string]string{model.DevLabel: "true"}
	if !reflect.DeepEqual(tr2.App.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong d2 labels: '%v'", tr2.App.ObjectMeta().Labels)
	}
	if !reflect.DeepEqual(tr2.App.TemplateObjectMeta().Labels, d2Orig.Spec.Template.Labels) {
		t.Fatalf("Wrong d2 pod labels: '%v'", tr2.App.TemplateObjectMeta().Labels)
	}
	expectedAnnotations = map[string]string{model.AppReplicasAnnotation: "3"}
	if !reflect.DeepEqual(tr2.App.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong d2 annotations: '%v'", tr2.App.ObjectMeta().Annotations)
	}
	if !reflect.DeepEqual(tr1.App.TemplateObjectMeta().Annotations, d2Orig.Spec.Template.Annotations) {
		t.Fatalf("Wrong d2 pod annotations: '%v'", tr2.App.TemplateObjectMeta().Annotations)
	}
	marshalledD2, _ := yaml.Marshal(tr2.App.PodSpec())
	marshalledD2Orig, _ := yaml.Marshal(d2Orig.Spec.Template.Spec)
	if string(marshalledD2) != string(marshalledD2Orig) {
		t.Fatalf("Wrong d2 generation.\nActual %+v, \nExpected %+v", string(marshalledD2), string(marshalledD2Orig))
	}

	//checking dev d2 state
	if tr2.DevApp.Replicas() != 3 {
		t.Fatalf("dev d2 is running %d replicas", tr2.DevApp.Replicas())
	}
	expectedLabels = map[string]string{model.DevCloneLabel: "deploy2"}
	if !reflect.DeepEqual(tr2.DevApp.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong dev d2 labels: '%v'", tr2.DevApp.ObjectMeta().Labels)
	}
	expectedPodLabels = map[string]string{"app": "worker", model.DetachedDevLabel: dev2.Name}
	if !reflect.DeepEqual(tr2.DevApp.TemplateObjectMeta().Labels, expectedPodLabels) {
		t.Fatalf("Wrong dev d2 pod labels: '%v'", tr2.DevApp.TemplateObjectMeta().Labels)
	}
	expectedAnnotations = map[string]string{"key2": "value2"}
	if !reflect.DeepEqual(tr2.DevApp.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong dev d2 annotations: '%v'", tr2.DevApp.ObjectMeta().Annotations)
	}
	expectedPodAnnotations = map[string]string{"key2": "value2"}
	if !reflect.DeepEqual(tr2.DevApp.TemplateObjectMeta().Annotations, expectedPodAnnotations) {
		t.Fatalf("Wrong dev d2 pod annotations: '%v'", tr2.DevApp.TemplateObjectMeta().Annotations)
	}
	marshalledDevD2, _ := yaml.Marshal(tr2.DevApp.PodSpec())
	marshalledDevD2OK, _ := yaml.Marshal(d2DevPodOK)
	if string(marshalledDevD2) != string(marshalledDevD2OK) {
		t.Fatalf("Wrong dev d2 generation.\nActual %+v, \nExpected %+v", string(marshalledDevD2), string(marshalledDevD2OK))
	}

	tr2.DevModeOff()

	if _, ok := tr2.App.ObjectMeta().Labels[model.DevLabel]; ok {
		t.Fatalf("'%s' label not eliminated on 'okteto down'", model.DevLabel)
	}

	if tr2.App.Replicas() != 3 {
		t.Fatalf("d2 is running %d replicas after 'okteto down'", tr2.App.Replicas())
	}
}

func Test_translateWithoutVolumes(t *testing.T) {
	manifest := []byte(`name: web
namespace: n
image: web:latest
sync:
  - .:/okteto
persistentVolume:
  enabled: false`)

	dev, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	d := deployments.Sandbox(dev)
	rule := dev.ToTranslationRule(dev, true)
	tr := &Translation{
		MainDev: dev,
		Dev:     dev,
		App:     NewDeploymentApp(d),
		Rules:   []*model.TranslationRule{rule},
	}
	if err := tr.translate(); err != nil {
		t.Fatal(err)
	}
	dDevPodOK := &apiv1.PodSpec{
		TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
		Volumes: []apiv1.Volume{
			{
				Name: oktetoSyncSecretVolume,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "okteto-web",
						Items: []apiv1.KeyToPath{
							{
								Key:  "config.xml",
								Path: "config.xml",
								Mode: &mode444,
							},
							{
								Key:  "cert.pem",
								Path: "cert.pem",
								Mode: &mode444,
							},
							{
								Key:  "key.pem",
								Path: "key.pem",
								Mode: &mode444,
							},
						},
					},
				},
			},
			{
				Name: dev.GetVolumeName(),
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: OktetoBinName,
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []apiv1.Container{
			{
				Name:            OktetoBinName,
				Image:           model.OktetoBinImageTag,
				ImagePullPolicy: apiv1.PullIfNotPresent,
				Command:         []string{"sh", "-c", "cp /usr/local/bin/* /okteto/bin"},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      OktetoBinName,
						MountPath: "/okteto/bin",
					},
				},
			},
		},
		Containers: []apiv1.Container{
			{
				Name:            "dev",
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullAlways,
				Command:         []string{"/var/okteto/bin/start.sh"},
				Args:            []string{"-r", "-e"},
				WorkingDir:      "",
				Env: []apiv1.EnvVar{
					{
						Name:  "OKTETO_NAMESPACE",
						Value: "n",
					},
					{
						Name:  "OKTETO_NAME",
						Value: "web",
					},
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/var/syncthing",
						SubPath:   model.SyncthingSubPath,
					},
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: model.RemoteMountPath,
						SubPath:   model.RemoteSubPath,
					},
					{
						Name:      oktetoSyncSecretVolume,
						ReadOnly:  false,
						MountPath: "/var/syncthing/secret/",
					},
					{
						Name:      OktetoBinName,
						ReadOnly:  false,
						MountPath: "/var/okteto/bin",
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
			},
		},
	}
	marshalledDev, _ := yaml.Marshal(tr.DevApp.PodSpec())
	marshalledDevOK, _ := yaml.Marshal(dDevPodOK)

	if string(marshalledDev) != string(marshalledDevOK) {
		t.Fatalf("Wrong d1 generation.\nActual %+v, \nExpected %+v", string(marshalledDev), string(marshalledDevOK))
	}
}

func Test_translateWithDocker(t *testing.T) {
	manifest := []byte(`name: web
namespace: n
image: web:latest
sync:
  - .:/app
docker:
  enabled: true
  image: docker:19
  resources:
    requests:
      cpu: 500m
      memory: 2Gi
    limits:
      cpu: 2
      memory: 4Gi`)

	dev, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}
	dev.Username = "cindy"
	dev.RegistryURL = "registry.okteto.dev"

	d := deployments.Sandbox(dev)
	rule := dev.ToTranslationRule(dev, false)
	tr := &Translation{
		MainDev: dev,
		Dev:     dev,
		App:     NewDeploymentApp(d),
		Rules:   []*model.TranslationRule{rule},
	}
	if err := tr.translate(); err != nil {
		t.Fatal(err)
	}
	dDevPodOK := apiv1.PodSpec{
		TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
		SecurityContext: &apiv1.PodSecurityContext{
			FSGroup: pointer.Int64Ptr(0),
		},
		Volumes: []apiv1.Volume{
			{
				Name: oktetoSyncSecretVolume,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "okteto-web",
						Items: []apiv1.KeyToPath{
							{
								Key:  "config.xml",
								Path: "config.xml",
								Mode: &mode444,
							},
							{
								Key:  "cert.pem",
								Path: "cert.pem",
								Mode: &mode444,
							},
							{
								Key:  "key.pem",
								Path: "key.pem",
								Mode: &mode444,
							},
						},
					},
				},
			},
			{
				Name: dev.GetVolumeName(),
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: dev.GetVolumeName(),
						ReadOnly:  false,
					},
				},
			},
			{
				Name: OktetoBinName,
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []apiv1.Container{
			{
				Name:            OktetoBinName,
				Image:           model.OktetoBinImageTag,
				ImagePullPolicy: apiv1.PullIfNotPresent,
				Command:         []string{"sh", "-c", "cp /usr/local/bin/* /okteto/bin"},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  pointer.Int64Ptr(0),
					RunAsGroup: pointer.Int64Ptr(0),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      OktetoBinName,
						MountPath: "/okteto/bin",
					},
				},
			},
			{
				Name:            OktetoInitVolumeContainerName,
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullIfNotPresent,
				Command:         []string{"sh", "-cx", "echo initializing && ( [ \"$(ls -A /init-volume/1)\" ] || cp -R /app/. /init-volume/1 || true)"},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  pointer.Int64Ptr(0),
					RunAsGroup: pointer.Int64Ptr(0),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/1",
						SubPath:   model.SourceCodeSubPath,
					},
				},
			},
		},
		Containers: []apiv1.Container{
			{
				Name:            "dev",
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullAlways,
				Command:         []string{"/var/okteto/bin/start.sh"},
				Args:            []string{"-r", "-d"},
				Env: []apiv1.EnvVar{
					{
						Name:  "OKTETO_NAMESPACE",
						Value: "n",
					},
					{
						Name:  "OKTETO_NAME",
						Value: "web",
					},
					{
						Name:  "OKTETO_USERNAME",
						Value: "cindy",
					},
					{
						Name:  "OKTETO_REGISTRY_URL",
						Value: "registry.okteto.dev",
					},
					{
						Name:  "DOCKER_HOST",
						Value: model.DefaultDockerHost,
					},
					{
						Name:  "DOCKER_CERT_PATH",
						Value: "/certs/client",
					},
					{
						Name:  "DOCKER_TLS_VERIFY",
						Value: "1",
					},
				},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  pointer.Int64Ptr(0),
					RunAsGroup: pointer.Int64Ptr(0),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: model.DefaultDockerCertDir,
						SubPath:   model.DefaultDockerCertDirSubPath,
					},
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/var/syncthing",
						SubPath:   model.SyncthingSubPath,
					},
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: model.RemoteMountPath,
						SubPath:   model.RemoteSubPath,
					},
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/app",
						SubPath:   model.SourceCodeSubPath,
					},
					{
						Name:      oktetoSyncSecretVolume,
						ReadOnly:  false,
						MountPath: "/var/syncthing/secret/",
					},
					{
						Name:      OktetoBinName,
						ReadOnly:  false,
						MountPath: "/var/okteto/bin",
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
			},
			{
				Name:  "dind",
				Image: "docker:19",
				Env: []apiv1.EnvVar{
					{
						Name:  "DOCKER_TLS_CERTDIR",
						Value: model.DefaultDockerCertDir,
					},
				},
				SecurityContext: &apiv1.SecurityContext{
					Privileged: pointer.BoolPtr(true),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: model.DefaultDockerCertDir,
						SubPath:   model.DefaultDockerCertDirSubPath,
					},
					{
						Name:      dev.GetVolumeName(),
						MountPath: model.DefaultDockerCacheDir,
						SubPath:   model.DefaultDockerCacheDirSubPath,
					},
					{
						Name:      dev.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/app",
						SubPath:   model.SourceCodeSubPath,
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						"cpu":    resource.MustParse("500"),
						"memory": resource.MustParse("2Gi"),
					},
					Limits: apiv1.ResourceList{
						"cpu":    resource.MustParse("2"),
						"memory": resource.MustParse("4Gi"),
					},
				},
			},
		},
	}
	marshalled, _ := yaml.Marshal(tr.DevApp.PodSpec())
	marshalledOK, _ := yaml.Marshal(dDevPodOK)
	if string(marshalled) != string(marshalledOK) {
		t.Fatalf("Wrong d generation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledOK))
	}
}

func Test_translateResources(t *testing.T) {
	type args struct {
		c *apiv1.Container
		r model.ResourceRequirements
	}
	tests := []struct {
		name             string
		args             args
		expectedRequests map[apiv1.ResourceName]resource.Quantity
		expectedLimits   map[apiv1.ResourceName]resource.Quantity
	}{
		{
			name: "no-limits-in-yaml",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{},
				},
				r: model.ResourceRequirements{},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{},
			expectedLimits:   map[apiv1.ResourceName]resource.Quantity{},
		},
		{
			name: "limits-in-yaml-no-limits-in-container",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{},
				},
				r: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
						apiv1.ResourceCPU:    resource.MustParse("0.125"),
					},
					Requests: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("2Gi"),
						apiv1.ResourceCPU:    resource.MustParse("1"),
					},
				},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("2Gi"),
				apiv1.ResourceCPU:    resource.MustParse("1"),
			},
			expectedLimits: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
				apiv1.ResourceCPU:    resource.MustParse("0.125"),
			},
		},
		{
			name: "no-limits-in-yaml-limits-in-container",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{
						Limits: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
							apiv1.ResourceCPU:    resource.MustParse("0.125"),
						},
						Requests: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("2Gi"),
							apiv1.ResourceCPU:    resource.MustParse("1"),
						},
					},
				},
				r: model.ResourceRequirements{},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("2Gi"),
				apiv1.ResourceCPU:    resource.MustParse("1"),
			},
			expectedLimits: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
				apiv1.ResourceCPU:    resource.MustParse("0.125"),
			},
		},
		{
			name: "limits-in-yaml-limits-in-container",
			args: args{
				c: &apiv1.Container{
					Resources: apiv1.ResourceRequirements{
						Limits: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("0.250Gi"),
							apiv1.ResourceCPU:    resource.MustParse("0.125"),
						},
						Requests: map[apiv1.ResourceName]resource.Quantity{
							apiv1.ResourceMemory: resource.MustParse("2Gi"),
							apiv1.ResourceCPU:    resource.MustParse("1"),
						},
					},
				},
				r: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("1Gi"),
						apiv1.ResourceCPU:    resource.MustParse("2"),
					},
					Requests: model.ResourceList{
						apiv1.ResourceMemory: resource.MustParse("4Gi"),
						apiv1.ResourceCPU:    resource.MustParse("0.125"),
					},
				},
			},
			expectedRequests: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("4Gi"),
				apiv1.ResourceCPU:    resource.MustParse("0.125"),
			},
			expectedLimits: map[apiv1.ResourceName]resource.Quantity{
				apiv1.ResourceMemory: resource.MustParse("1Gi"),
				apiv1.ResourceCPU:    resource.MustParse("2"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			TranslateResources(tt.args.c, tt.args.r)

			a := tt.args.c.Resources.Requests[apiv1.ResourceMemory]
			b := tt.expectedRequests[apiv1.ResourceMemory]

			if a.Cmp(b) != 0 {
				t.Errorf("requests %s: expected %s, got %s", apiv1.ResourceMemory, b.String(), a.String())
			}

			a = tt.args.c.Resources.Requests[apiv1.ResourceCPU]
			b = tt.expectedRequests[apiv1.ResourceCPU]

			if a.Cmp(b) != 0 {
				t.Errorf("requests %s: expected %s, got %s", apiv1.ResourceCPU, b.String(), a.String())
			}

			a = tt.args.c.Resources.Limits[apiv1.ResourceMemory]
			b = tt.expectedLimits[apiv1.ResourceMemory]

			if a.Cmp(b) != 0 {
				t.Errorf("limits %s: expected %s, got %s", apiv1.ResourceMemory, b.String(), a.String())
			}

			a = tt.args.c.Resources.Limits[apiv1.ResourceCPU]
			b = tt.expectedLimits[apiv1.ResourceCPU]

			if a.Cmp(b) != 0 {
				t.Errorf("limits %s: expected %s, got %s", apiv1.ResourceCPU, b.String(), a.String())
			}
		})
	}
}

func Test_translateSecurityContext(t *testing.T) {
	var trueB = true

	tests := []struct {
		name         string
		c            *apiv1.Container
		s            *model.SecurityContext
		expectedAdd  []apiv1.Capability
		expectedDrop []apiv1.Capability
	}{
		{
			name: "single-add",
			c:    &apiv1.Container{},
			s: &model.SecurityContext{
				Capabilities: &model.Capabilities{
					Add: []apiv1.Capability{"SYS_TRACE"},
				},
			},
			expectedAdd: []apiv1.Capability{"SYS_TRACE"},
		},
		{
			name: "add-drop",
			c:    &apiv1.Container{},
			s: &model.SecurityContext{
				Capabilities: &model.Capabilities{
					Add:  []apiv1.Capability{"SYS_TRACE"},
					Drop: []apiv1.Capability{"SYS_NICE"},
				},
			},
			expectedAdd:  []apiv1.Capability{"SYS_TRACE"},
			expectedDrop: []apiv1.Capability{"SYS_NICE"},
		},
		{
			name: "merge-uniques",
			c: &apiv1.Container{
				SecurityContext: &apiv1.SecurityContext{
					Capabilities: &apiv1.Capabilities{
						Add:  []apiv1.Capability{"SYS_FOO"},
						Drop: []apiv1.Capability{"SYS_BAR"},
					},
				},
			},
			s: &model.SecurityContext{
				Capabilities: &model.Capabilities{
					Add:  []apiv1.Capability{"SYS_TRACE"},
					Drop: []apiv1.Capability{"SYS_NICE"},
				},
			},
			expectedAdd:  []apiv1.Capability{"SYS_FOO", "SYS_TRACE"},
			expectedDrop: []apiv1.Capability{"SYS_BAR", "SYS_NICE"},
		},
		{
			name: "read-only",
			c: &apiv1.Container{
				SecurityContext: &apiv1.SecurityContext{
					ReadOnlyRootFilesystem: &trueB,
				},
			},
			s: &model.SecurityContext{
				Capabilities: &model.Capabilities{
					Add: []apiv1.Capability{"SYS_TRACE"},
				},
			},
			expectedAdd: []apiv1.Capability{"SYS_TRACE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			TranslateContainerSecurityContext(tt.c, tt.s)
			if tt.c.SecurityContext == nil {
				t.Fatal("SecurityContext was nil")
			}

			if !reflect.DeepEqual(tt.c.SecurityContext.Capabilities.Add, tt.expectedAdd) {
				t.Errorf("tt.c.SecurityContext.Capabilities.Add != tt.expectedAdd. Expected: %s, Got; %s", tt.expectedAdd, tt.c.SecurityContext.Capabilities.Add)
			}

			if !reflect.DeepEqual(tt.c.SecurityContext.Capabilities.Drop, tt.expectedDrop) {
				t.Errorf("tt.c.SecurityContext.Capabilities.Drop != tt.expectedDrop. Expected: %s, Got; %s", tt.expectedDrop, tt.c.SecurityContext.Capabilities.Drop)
			}

			if tt.c.SecurityContext.ReadOnlyRootFilesystem != nil {
				t.Errorf("ReadOnlyRootFilesystem was not removed")
			}
		})
	}
}

func TestTranslateOktetoVolumes(t *testing.T) {
	var tests = []struct {
		name     string
		spec     *apiv1.PodSpec
		rule     *model.TranslationRule
		expected []apiv1.Volume
	}{
		{
			name: "single-persistence-enabled",
			spec: &apiv1.PodSpec{},
			rule: &model.TranslationRule{
				PersistentVolume: true,
				Volumes: []model.VolumeMount{
					{Name: "okteto"},
				},
			},
			expected: []apiv1.Volume{
				{
					Name: "okteto",
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "okteto",
							ReadOnly:  false,
						},
					},
				},
			},
		},
		{
			name: "single-persistence-disabled",
			spec: &apiv1.PodSpec{},
			rule: &model.TranslationRule{
				PersistentVolume: false,
				Volumes: []model.VolumeMount{
					{
						Name:      "okteto",
						SubPath:   model.SyncthingSubPath,
						MountPath: model.OktetoSyncthingMountPath,
					},
				},
			},
			expected: []apiv1.Volume{
				{
					Name:         "okteto",
					VolumeSource: apiv1.VolumeSource{EmptyDir: &apiv1.EmptyDirVolumeSource{}},
				},
			},
		},
		{
			name: "external-volume",
			spec: &apiv1.PodSpec{},
			rule: &model.TranslationRule{
				PersistentVolume: false,
				Volumes: []model.VolumeMount{
					{
						Name: "okteto",
					},
				},
			},
			expected: []apiv1.Volume{
				{
					Name: "okteto",
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "okteto",
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			TranslateOktetoVolumes(tt.spec, tt.rule)
			if !reflect.DeepEqual(tt.expected, tt.spec.Volumes) {
				t.Errorf("Expected \n%+v but got \n%+v", tt.expected, tt.spec.Volumes)
			}
		})
	}
}

func Test_translateMultipleEnvVars(t *testing.T) {
	manifest := []byte(`name: web
namespace: n
image: web:latest
sync:
  - .:/app
environment:
  key2: value2
  key1: value1
  key4: value4
  key5: value5
  key3: value3
`)

	dev, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}
	dev.Username = "cindy"

	d := deployments.Sandbox(dev)
	rule := dev.ToTranslationRule(dev, false)
	tr := &Translation{
		MainDev: dev,
		Dev:     dev,
		App:     NewDeploymentApp(d),
		Rules:   []*model.TranslationRule{rule},
	}
	if err := tr.translate(); err != nil {
		t.Fatal(err)
	}
	envOK := []apiv1.EnvVar{
		{
			Name:  "key1",
			Value: "value1",
		},
		{
			Name:  "key2",
			Value: "value2",
		},
		{
			Name:  "key3",
			Value: "value3",
		},
		{
			Name:  "key4",
			Value: "value4",
		},
		{
			Name:  "key5",
			Value: "value5",
		},
		{
			Name:  "OKTETO_NAMESPACE",
			Value: "n",
		},
		{
			Name:  "OKTETO_NAME",
			Value: "web",
		},
		{
			Name:  "OKTETO_USERNAME",
			Value: "cindy",
		},
	}
	if !reflect.DeepEqual(envOK, tr.DevApp.PodSpec().Containers[0].Env) {
		t.Fatalf("Wrong env generation %+v", tr.DevApp.PodSpec().Containers[0].Env)
	}
}

func Test_translateSfsWithVolumes(t *testing.T) {
	file, err := os.CreateTemp("/tmp", "okteto-secret-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	var runAsUser int64 = 100
	var runAsGroup int64 = 101
	var fsGroup int64 = 102
	manifest := []byte(fmt.Sprintf(`name: web
namespace: n
container: dev
image: web:latest
command: ["./run_web.sh"]
workdir: /app
annotations:
  key1: value1
tolerations:
- key: nvidia/gpu
  operator: Exists
securityContext:
  runAsUser: 100
  runAsGroup: 101
  fsGroup: 102
serviceAccount: sa
sync:
  - .:/app
  - sub:/path
volumes:
  - /go/pkg/
  - /root/.cache/go-build
nodeSelector:
  disktype: ssd
affinity:
  podAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: role
          operator: In
          values:
          - web-server
      topologyKey: kubernetes.io/hostname
secrets:
  - %s:/remote
resources:
  limits:
    cpu: 2
    memory: 1Gi
    nvidia.com/gpu: 1
    amd.com/gpu: 1
services:
  - name: worker
    image: worker:latest
    annotations:
      key2: value2
    command: ["./run_worker.sh"]
    sync:
       - worker:/src`, file.Name()))

	dev1, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	sfs1 := statefulsets.Sandbox(dev1)
	sfs1.UID = types.UID("sfs1")
	delete(sfs1.Annotations, model.OktetoAutoCreateAnnotation)
	sfs1.Spec.Replicas = pointer.Int32Ptr(2)

	rule1 := dev1.ToTranslationRule(dev1, false)
	tr1 := &Translation{
		MainDev: dev1,
		Dev:     dev1,
		App:     NewStatefulSetApp(sfs1),
		Rules:   []*model.TranslationRule{rule1},
	}
	err = tr1.translate()
	if err != nil {
		t.Fatal(err)
	}
	sfs1PodDev := apiv1.PodSpec{
		NodeSelector: map[string]string{
			"disktype": "ssd",
		},
		Affinity: &apiv1.Affinity{
			PodAffinity: &apiv1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "role",
									Operator: "In",
									Values: []string{
										"web-server",
									},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
		Tolerations: []apiv1.Toleration{
			{
				Key:      "nvidia/gpu",
				Operator: apiv1.TolerationOpExists,
			},
		},
		SecurityContext: &apiv1.PodSecurityContext{
			FSGroup: &fsGroup,
		},
		ServiceAccountName:            "sa",
		TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
		Volumes: []apiv1.Volume{
			{
				Name: oktetoSyncSecretVolume,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "okteto-web",
						Items: []apiv1.KeyToPath{
							{
								Key:  "config.xml",
								Path: "config.xml",
								Mode: &mode444,
							},
							{
								Key:  "cert.pem",
								Path: "cert.pem",
								Mode: &mode444,
							},
							{
								Key:  "key.pem",
								Path: "key.pem",
								Mode: &mode444,
							},
						},
					},
				},
			},
			{
				Name: dev1.GetVolumeName(),
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: dev1.GetVolumeName(),
						ReadOnly:  false,
					},
				},
			},
			{
				Name: oktetoDevSecretVolume,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "okteto-web",
						Items: []apiv1.KeyToPath{
							{
								Key:  "dev-secret-remote",
								Path: "remote",
								Mode: &mode420,
							},
						},
					},
				},
			},
			{
				Name: OktetoBinName,
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []apiv1.Container{
			{
				Name:            OktetoBinName,
				Image:           model.OktetoBinImageTag,
				ImagePullPolicy: apiv1.PullIfNotPresent,
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  &runAsUser,
					RunAsGroup: &runAsGroup,
				},
				Command: []string{"sh", "-c", "cp /usr/local/bin/* /okteto/bin"},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      OktetoBinName,
						MountPath: "/okteto/bin",
					},
				},
			},
			{
				Name:            OktetoInitVolumeContainerName,
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullIfNotPresent,
				Command:         []string{"sh", "-cx", "echo initializing && ( [ \"$(ls -A /init-volume/1)\" ] || cp -R /go/pkg/. /init-volume/1 || true) && ( [ \"$(ls -A /init-volume/2)\" ] || cp -R /root/.cache/go-build/. /init-volume/2 || true) && ( [ \"$(ls -A /init-volume/3)\" ] || cp -R /app/. /init-volume/3 || true) && ( [ \"$(ls -A /init-volume/4)\" ] || cp -R /path/. /init-volume/4 || true)"},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  &runAsUser,
					RunAsGroup: &runAsGroup,
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/1",
						SubPath:   path.Join(model.DataSubPath, "go/pkg"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/2",
						SubPath:   path.Join(model.DataSubPath, "root/.cache/go-build"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/3",
						SubPath:   model.SourceCodeSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/init-volume/4",
						SubPath:   path.Join(model.SourceCodeSubPath, "sub"),
					},
				},
			},
		},
		Containers: []apiv1.Container{
			{
				Name:            "dev",
				Image:           "web:latest",
				ImagePullPolicy: apiv1.PullAlways,
				Command:         []string{"/var/okteto/bin/start.sh"},
				Args:            []string{"-r", "-s", "remote:/remote"},
				WorkingDir:      "/app",
				Env: []apiv1.EnvVar{
					{
						Name:  "OKTETO_NAMESPACE",
						Value: "n",
					},
					{
						Name:  "OKTETO_NAME",
						Value: "web",
					},
				},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  &runAsUser,
					RunAsGroup: &runAsGroup,
				},
				Resources: apiv1.ResourceRequirements{
					Limits: apiv1.ResourceList{
						"cpu":            resource.MustParse("2"),
						"memory":         resource.MustParse("1Gi"),
						"nvidia.com/gpu": resource.MustParse("1"),
						"amd.com/gpu":    resource.MustParse("1"),
					},
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/var/syncthing",
						SubPath:   model.SyncthingSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: model.RemoteMountPath,
						SubPath:   model.RemoteSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/go/pkg/",
						SubPath:   path.Join(model.DataSubPath, "go/pkg"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/root/.cache/go-build",
						SubPath:   path.Join(model.DataSubPath, "root/.cache/go-build"),
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/app",
						SubPath:   model.SourceCodeSubPath,
					},
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/path",
						SubPath:   path.Join(model.SourceCodeSubPath, "sub"),
					},
					{
						Name:      oktetoSyncSecretVolume,
						ReadOnly:  false,
						MountPath: "/var/syncthing/secret/",
					},
					{
						Name:      oktetoDevSecretVolume,
						ReadOnly:  false,
						MountPath: "/var/okteto/secret/",
					},
					{
						Name:      OktetoBinName,
						ReadOnly:  false,
						MountPath: "/var/okteto/bin",
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
			},
		},
	}

	//checking sfs1 state
	sfs1Orig := statefulsets.Sandbox(dev1)
	if tr1.App.Replicas() != 0 {
		t.Fatalf("sfs1 is running %d replicas", tr1.App.Replicas())
	}
	expectedLabels := map[string]string{model.DevLabel: "true"}
	if !reflect.DeepEqual(tr1.App.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong sfs1 labels: '%v'", tr1.App.ObjectMeta().Labels)
	}
	if !reflect.DeepEqual(tr1.App.TemplateObjectMeta().Labels, sfs1Orig.Spec.Template.Labels) {
		t.Fatalf("Wrong sfs1 pod labels: '%v'", tr1.App.TemplateObjectMeta().Labels)
	}
	expectedAnnotations := map[string]string{model.AppReplicasAnnotation: "2"}
	if !reflect.DeepEqual(tr1.App.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong sfs1 annotations: '%v'", tr1.App.ObjectMeta().Annotations)
	}
	if !reflect.DeepEqual(tr1.App.TemplateObjectMeta().Annotations, sfs1Orig.Spec.Template.Annotations) {
		t.Fatalf("Wrong sfs1 pod annotations: '%v'", tr1.App.TemplateObjectMeta().Annotations)
	}
	marshalledSfs1, _ := yaml.Marshal(tr1.App.PodSpec())
	marshalledSfs1Orig, _ := yaml.Marshal(sfs1Orig.Spec.Template.Spec)
	if string(marshalledSfs1) != string(marshalledSfs1Orig) {
		t.Fatalf("Wrong sfs1 generation.\nActual %+v, \nExpected %+v", string(marshalledSfs1), string(marshalledSfs1Orig))
	}

	//checking dev sfs1 state
	if tr1.DevApp.Replicas() != 1 {
		t.Fatalf("dev sfs1 is running %d replicas", tr1.DevApp.Replicas())
	}
	expectedLabels = map[string]string{model.DevCloneLabel: "sfs1"}
	if !reflect.DeepEqual(tr1.DevApp.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong dev sfs1 labels: '%v'", tr1.DevApp.ObjectMeta().Labels)
	}
	expectedPodLabels := map[string]string{"app": "web", model.InteractiveDevLabel: dev1.Name}
	if !reflect.DeepEqual(tr1.DevApp.TemplateObjectMeta().Labels, expectedPodLabels) {
		t.Fatalf("Wrong dev sfs1 pod labels: '%v'", tr1.DevApp.TemplateObjectMeta().Labels)
	}
	expectedAnnotations = map[string]string{"key1": "value1"}
	if !reflect.DeepEqual(tr1.DevApp.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong dev sfs1 annotations: '%v'", tr1.DevApp.ObjectMeta().Annotations)
	}
	expectedPodAnnotations := map[string]string{"key1": "value1"}
	if !reflect.DeepEqual(tr1.DevApp.TemplateObjectMeta().Annotations, expectedPodAnnotations) {
		t.Fatalf("Wrong dev sfs1 pod annotations: '%v'", tr1.DevApp.TemplateObjectMeta().Annotations)
	}
	marshalledDevSfs1, _ := yaml.Marshal(tr1.DevApp.PodSpec())
	marshalledDevSfs1OK, _ := yaml.Marshal(sfs1PodDev)
	if string(marshalledDevSfs1) != string(marshalledDevSfs1OK) {
		t.Fatalf("Wrong dev sfs1 generation.\nActual %+v, \nExpected %+v", string(marshalledDevSfs1), string(marshalledDevSfs1OK))
	}

	tr1.DevModeOff()

	if _, ok := tr1.App.ObjectMeta().Labels[model.DevLabel]; ok {
		t.Fatalf("'%s' label not eliminated on 'okteto down'", model.DevLabel)
	}

	if tr1.App.Replicas() != 2 {
		t.Fatalf("sfs1 is running %d replicas after 'okteto down'", tr1.App.Replicas())
	}

	dev2 := dev1.Services[0]
	sfs2 := statefulsets.Sandbox(dev2)
	sfs2.Spec.Replicas = pointer.Int32Ptr(3)
	sfs2.UID = types.UID("sfs2")
	delete(sfs2.Annotations, model.OktetoAutoCreateAnnotation)
	sfs2.Namespace = dev1.Namespace

	trMap := make(map[string]*Translation)
	ctx := context.Background()
	c := fake.NewSimpleClientset(sfs2)
	err = loadServiceTranslations(ctx, dev1, false, trMap, c)
	if err != nil {
		t.Fatal(err)
	}
	tr2 := trMap[dev2.Name]
	err = tr2.translate()
	if err != nil {
		t.Fatal(err)
	}
	sfs2DevPod := apiv1.PodSpec{
		Affinity: &apiv1.Affinity{
			PodAffinity: &apiv1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []apiv1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								model.InteractiveDevLabel: "web",
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
		SecurityContext: &apiv1.PodSecurityContext{
			FSGroup: pointer.Int64Ptr(0),
		},
		ServiceAccountName:            "",
		TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
		Volumes: []apiv1.Volume{
			{
				Name: dev1.GetVolumeName(),
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: dev1.GetVolumeName(),
						ReadOnly:  false,
					},
				},
			},
		},
		Containers: []apiv1.Container{
			{
				Name:            "dev",
				Image:           "worker:latest",
				ImagePullPolicy: apiv1.PullAlways,
				Command:         []string{"./run_worker.sh"},
				Args:            []string{},
				SecurityContext: &apiv1.SecurityContext{
					RunAsUser:  pointer.Int64Ptr(0),
					RunAsGroup: pointer.Int64Ptr(0),
				},
				VolumeMounts: []apiv1.VolumeMount{
					{
						Name:      dev1.GetVolumeName(),
						ReadOnly:  false,
						MountPath: "/src",
						SubPath:   path.Join(model.SourceCodeSubPath, "worker"),
					},
				},
				LivenessProbe:  nil,
				ReadinessProbe: nil,
			},
		},
	}

	//checking sfs2 state
	sfs2Orig := statefulsets.Sandbox(dev2)
	if tr2.App.Replicas() != 0 {
		t.Fatalf("sfs2 is running %d replicas", tr2.App.Replicas())
	}
	expectedLabels = map[string]string{model.DevLabel: "true"}
	if !reflect.DeepEqual(tr2.App.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong sfs2 labels: '%v'", tr2.App.ObjectMeta().Labels)
	}
	if !reflect.DeepEqual(tr2.App.TemplateObjectMeta().Labels, sfs2Orig.Spec.Template.Labels) {
		t.Fatalf("Wrong sfs2 pod labels: '%v'", tr2.App.TemplateObjectMeta().Labels)
	}
	expectedAnnotations = map[string]string{model.AppReplicasAnnotation: "3"}
	if !reflect.DeepEqual(tr2.App.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong sfs2 annotations: '%v'", tr2.App.ObjectMeta().Annotations)
	}
	if !reflect.DeepEqual(tr1.App.TemplateObjectMeta().Annotations, sfs2Orig.Spec.Template.Annotations) {
		t.Fatalf("Wrong sfs2 pod annotations: '%v'", tr2.App.TemplateObjectMeta().Annotations)
	}
	marshalledSfs2, _ := yaml.Marshal(tr2.App.PodSpec())
	marshalledSfs2Orig, _ := yaml.Marshal(sfs2Orig.Spec.Template.Spec)
	if string(marshalledSfs2) != string(marshalledSfs2Orig) {
		t.Fatalf("Wrong sfs2 generation.\nActual %+v, \nExpected %+v", string(marshalledSfs2), string(marshalledSfs2Orig))
	}

	//checking dev sfs2 state
	if tr2.DevApp.Replicas() != 3 {
		t.Fatalf("dev sfs2 is running %d replicas", tr2.DevApp.Replicas())
	}
	expectedLabels = map[string]string{model.DevCloneLabel: "sfs2"}
	if !reflect.DeepEqual(tr2.DevApp.ObjectMeta().Labels, expectedLabels) {
		t.Fatalf("Wrong dev sfs2 labels: '%v'", tr2.DevApp.ObjectMeta().Labels)
	}
	expectedPodLabels = map[string]string{"app": "worker", model.DetachedDevLabel: dev2.Name}
	if !reflect.DeepEqual(tr2.DevApp.TemplateObjectMeta().Labels, expectedPodLabels) {
		t.Fatalf("Wrong dev sfs2 pod labels: '%v'", tr2.DevApp.TemplateObjectMeta().Labels)
	}
	expectedAnnotations = map[string]string{"key2": "value2"}
	if !reflect.DeepEqual(tr2.DevApp.ObjectMeta().Annotations, expectedAnnotations) {
		t.Fatalf("Wrong dev sfs2 annotations: '%v'", tr2.DevApp.ObjectMeta().Annotations)
	}
	expectedPodAnnotations = map[string]string{"key2": "value2"}
	if !reflect.DeepEqual(tr2.DevApp.TemplateObjectMeta().Annotations, expectedPodAnnotations) {
		t.Fatalf("Wrong dev sfs2 pod annotations: '%v'", tr2.DevApp.TemplateObjectMeta().Annotations)
	}
	marshalledDevSfs2, _ := yaml.Marshal(tr2.DevApp.PodSpec())
	marshalledDevSfs2OK, _ := yaml.Marshal(sfs2DevPod)
	if string(marshalledDevSfs2) != string(marshalledDevSfs2OK) {
		t.Fatalf("Wrong dev sfs2 generation.\nActual %+v, \nExpected %+v", string(marshalledDevSfs2), string(marshalledDevSfs2OK))
	}

	tr2.DevModeOff()

	if _, ok := tr2.App.ObjectMeta().Labels[model.DevLabel]; ok {
		t.Fatalf("'%s' label not eliminated on 'okteto down'", model.DevLabel)
	}

	if tr2.App.Replicas() != 3 {
		t.Fatalf("sfs2 is running %d replicas after 'okteto down'", tr2.App.Replicas())
	}
}
