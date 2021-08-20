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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var (
	rootUser int64
	mode444  int32 = 0444
	mode420  int32 = 420
)

func Test_translateWithVolumes(t *testing.T) {
	file, err := ioutil.TempFile("/tmp", "okteto-secret-test")
	if err != nil {
		log.Fatal(err)
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
    serviceAccount: sa
    sync:
       - worker:/src`, file.Name()))

	dev, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	d1 := model.NewResource(dev)
	d1.GetSandbox()
	d1.SetReplicas(pointer.Int32Ptr(2))
	d1.Deployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
	}
	rule1 := dev.ToTranslationRule(dev, false)
	tr1 := &model.Translation{
		Interactive: true,
		Name:        dev.Name,
		Version:     model.TranslationVersion,
		K8sObject:   d1,
		Rules:       []*model.TranslationRule{rule1},
		Replicas:    2,
		Strategy: model.K8sObjectStrategy{
			DeploymentStrategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
		Annotations: model.Annotations{"key": "value"},
		Tolerations: []apiv1.Toleration{
			{
				Key:      "nvidia/cpu",
				Operator: apiv1.TolerationOpExists,
			},
		},
	}
	err = translate(tr1, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	d1OK := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Tolerations: []apiv1.Toleration{
						{
							Key:      "nvidia/cpu",
							Operator: apiv1.TolerationOpExists,
						},
					},
					SecurityContext: &apiv1.PodSecurityContext{
						FSGroup: &fsGroup,
					},
					ServiceAccountName:            "sa",
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
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
									Name:      dev.GetVolumeName(),
									ReadOnly:  false,
									MountPath: "/init-volume/1",
									SubPath:   path.Join(model.DataSubPath, "go/pkg"),
								},
								{
									Name:      dev.GetVolumeName(),
									ReadOnly:  false,
									MountPath: "/init-volume/2",
									SubPath:   path.Join(model.DataSubPath, "root/.cache/go-build"),
								},
								{
									Name:      dev.GetVolumeName(),
									ReadOnly:  false,
									MountPath: "/init-volume/3",
									SubPath:   model.SourceCodeSubPath,
								},
								{
									Name:      dev.GetVolumeName(),
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
									MountPath: "/go/pkg/",
									SubPath:   path.Join(model.DataSubPath, "go/pkg"),
								},
								{
									Name:      dev.GetVolumeName(),
									ReadOnly:  false,
									MountPath: "/root/.cache/go-build",
									SubPath:   path.Join(model.DataSubPath, "root/.cache/go-build"),
								},
								{
									Name:      dev.GetVolumeName(),
									ReadOnly:  false,
									MountPath: "/app",
									SubPath:   model.SourceCodeSubPath,
								},
								{
									Name:      dev.GetVolumeName(),
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
				},
			},
		},
	}
	marshalled1, _ := yaml.Marshal(d1.Deployment.Spec.Template.Spec)
	marshalled1OK, _ := yaml.Marshal(d1OK.Spec.Template.Spec)
	if string(marshalled1) != string(marshalled1OK) {
		t.Fatalf("Wrong d1 generation.\nActual %+v, \nExpected %+v", string(marshalled1), string(marshalled1OK))
	}
	if d1.GetAnnotation("key") != "value" {
		t.Fatalf("Wrong d1 annotations: '%s'", d1.GetAnnotation("key"))
	}
	if d1.PodTemplateSpec.Annotations["key"] != "value" {
		t.Fatalf("Wrong d1 pod annotations: '%s'", d1.PodTemplateSpec.Annotations["key"])
	}

	d1Down, err := TranslateDevModeOff(d1)
	if err != nil {
		t.Fatal(err)
	}

	d1Orig := model.NewResource(dev)
	d1Orig.GetSandbox()
	marshalled1Down, _ := yaml.Marshal(d1Down.PodTemplateSpec.Spec)
	marshalled1Orig, _ := yaml.Marshal(d1Orig.PodTemplateSpec.Spec)
	if string(marshalled1Down) != string(marshalled1Orig) {
		t.Fatalf("Wrong d1 down.\nActual %+v, \nExpected %+v", string(marshalled1Down), string(marshalled1Orig))
	}
	if d1Down.GetAnnotation("key") != "" {
		t.Fatalf("Wrong d1 annotations after down: '%s'", d1.GetAnnotation("key"))
	}
	if d1Down.PodTemplateSpec.Annotations["key"] != "" {
		t.Fatalf("Wrong d1 pod annotations after down: '%s'", d1.PodTemplateSpec.Annotations["key"])
	}
	if *d1Down.Replicas != 2 {
		t.Fatalf("Wrong d1 replicas %d vs 2", *d1Down.Replicas)
	}

	dev2 := dev.Services[0]
	d2 := model.NewResource(dev2)
	d2.GetSandbox()
	rule2 := dev2.ToTranslationRule(dev, false)
	tr2 := &model.Translation{
		Interactive: false,
		Name:        dev.Name,
		Version:     model.TranslationVersion,
		K8sObject:   d2,
		Rules:       []*model.TranslationRule{rule2},
		Annotations: model.Annotations{"key": "value"},
		Tolerations: []apiv1.Toleration{
			{
				Key:      "nvidia/cpu",
				Operator: apiv1.TolerationOpExists,
			},
		},
	}
	err = translate(tr2, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	d2OK := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
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
					Tolerations: []apiv1.Toleration{
						{
							Key:      "nvidia/cpu",
							Operator: apiv1.TolerationOpExists,
						},
					},
					SecurityContext: &apiv1.PodSecurityContext{
						FSGroup: &rootUser,
					},
					ServiceAccountName:            "sa",
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
					Volumes: []apiv1.Volume{
						{
							Name: dev.GetVolumeName(),
							VolumeSource: apiv1.VolumeSource{
								PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
									ClaimName: dev.GetVolumeName(),
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
								RunAsUser:    &rootUser,
								RunAsGroup:   &rootUser,
								RunAsNonRoot: &falseBoolean,
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      dev.GetVolumeName(),
									ReadOnly:  false,
									MountPath: "/src",
									SubPath:   path.Join(model.SourceCodeSubPath, "worker"),
								},
							},
							LivenessProbe:  nil,
							ReadinessProbe: nil,
						},
					},
				},
			},
		},
	}
	marshalled2, _ := yaml.Marshal(d2.PodTemplateSpec.Spec)
	marshalled2OK, _ := yaml.Marshal(d2OK.Spec.Template.Spec)
	if string(marshalled2) != string(marshalled2OK) {
		t.Fatalf("Wrong d2 generation.\nActual %s, \nExpected %s", string(marshalled2), string(marshalled2OK))
	}
	if d2.GetAnnotation("key") != "value" {
		t.Fatalf("Wrong d2 annotations: '%s'", d2.GetAnnotation("key"))
	}
	if d2.PodTemplateSpec.Annotations["key"] != "value" {
		t.Fatalf("Wrong d2 pod annotations: '%s'", d2.PodTemplateSpec.Annotations["key"])
	}

	d2Down, err := TranslateDevModeOff(d2)
	if err != nil {
		t.Fatal(err)
	}

	d2Orig := model.NewResource(dev2)
	d2.GetSandbox()
	marshalled2Down, _ := yaml.Marshal(d2Down.PodTemplateSpec.Spec)
	marshalled2Orig, _ := yaml.Marshal(d2Orig.PodTemplateSpec.Spec)
	if string(marshalled2Down) != string(marshalled2Orig) {
		t.Fatalf("Wrong d2 down.\nActual %+v, \nExpected %+v", string(marshalled2Down), string(marshalled2Orig))
	}
	if d2Down.GetAnnotation("key") != "" {
		t.Fatalf("Wrong d2 annotations after down: '%s'", d2.GetAnnotation("key"))
	}
	if d2Down.PodTemplateSpec.Annotations["key"] != "" {
		t.Fatalf("Wrong d2 pod annotations after down: '%s'", d2.PodTemplateSpec.Annotations["key"])
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

	d1 := model.NewResource(dev)
	d1.GetSandbox()
	rule1 := dev.ToTranslationRule(dev, true)
	tr1 := &model.Translation{
		Interactive: true,
		Name:        dev.Name,
		Version:     model.TranslationVersion,
		K8sObject:   d1,
		Rules:       []*model.TranslationRule{rule1},
	}
	err = translate(tr1, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	d1OK := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
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
				},
			},
		},
	}
	marshalled1, _ := yaml.Marshal(d1.PodTemplateSpec.Spec)
	marshalled1OK, _ := yaml.Marshal(d1OK.Spec.Template.Spec)

	if string(marshalled1) != string(marshalled1OK) {
		t.Fatalf("Wrong d1 generation.\nActual %+v, \nExpected %+v", string(marshalled1), string(marshalled1OK))
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

	d := model.NewResource(dev)
	d.GetSandbox()
	rule := dev.ToTranslationRule(dev, false)
	tr := &model.Translation{
		Interactive: true,
		Name:        dev.Name,
		Version:     model.TranslationVersion,
		K8sObject:   d,
		Rules:       []*model.TranslationRule{rule},
	}
	err = translate(tr, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	dOK := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
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
								RunAsUser:    &rootUser,
								RunAsGroup:   &rootUser,
								RunAsNonRoot: &falseBoolean,
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
								RunAsUser:    &rootUser,
								RunAsGroup:   &rootUser,
								RunAsNonRoot: &falseBoolean,
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
				},
			},
		},
	}
	marshalled, _ := yaml.Marshal(d.PodTemplateSpec.Spec)
	marshalledOK, _ := yaml.Marshal(dOK.Spec.Template.Spec)
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
	dev.RegistryURL = "registry.okteto.dev"

	d := model.NewResource(dev)
	d.GetSandbox()
	rule := dev.ToTranslationRule(dev, false)
	tr := &model.Translation{
		Interactive: true,
		Name:        dev.Name,
		Version:     model.TranslationVersion,
		K8sObject:   d,
		Rules:       []*model.TranslationRule{rule},
	}
	err = translate(tr, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	dOK := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &devTerminationGracePeriodSeconds,
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
								RunAsUser:    &rootUser,
								RunAsGroup:   &rootUser,
								RunAsNonRoot: &falseBoolean,
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
							Args:            []string{"-r"},
							Env: []apiv1.EnvVar{
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
							},
							SecurityContext: &apiv1.SecurityContext{
								RunAsUser:    &rootUser,
								RunAsGroup:   &rootUser,
								RunAsNonRoot: &falseBoolean,
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
					},
				},
			},
		},
	}
	marshalled, _ := yaml.Marshal(d.PodTemplateSpec.Spec)
	marshalledOK, _ := yaml.Marshal(dOK.Spec.Template.Spec)
	if string(marshalled) != string(marshalledOK) {
		t.Fatalf("Wrong d generation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledOK))
	}
}
