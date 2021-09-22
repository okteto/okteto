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

package model

import (
	"path"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/log"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDevToTranslationRule(t *testing.T) {
	manifest := []byte(`name: web
namespace: n
container: dev
image: web:latest
command: ["./run_web.sh"]
imagePullPolicy: Never
sync:
  - .:/app
  - sub:/path
resources:
  limits:
    cpu: 2
    memory: 1Gi
    nvidia.com/gpu: 1
    amd.com/gpu: 1
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
services:
  - name: worker
    container: dev
    image: worker:latest
    imagePullPolicy: IfNotPresent
    sync:
      - worker:/src
    healthchecks: true
    lifecycle:
       postStart: true`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	rule1 := dev.ToTranslationRule(dev, false)
	rule1OK := &TranslationRule{
		Marker:            OktetoBinImageTag,
		OktetoBinImageTag: OktetoBinImageTag,
		Container:         "dev",
		Image:             "web:latest",
		ImagePullPolicy:   apiv1.PullNever,
		Command:           []string{"/var/okteto/bin/start.sh"},
		Args:              []string{"-r"},
		Probes:            &Probes{},
		Lifecycle:         &Lifecycle{},
		Environment: Environment{
			{
				Name:  "OKTETO_NAMESPACE",
				Value: "n",
			},
			{
				Name:  "OKTETO_NAME",
				Value: "web",
			},
		},
		SecurityContext: &SecurityContext{
			RunAsUser:  &rootUser,
			RunAsGroup: &rootUser,
			FSGroup:    &rootUser,
		},
		Resources: ResourceRequirements{
			Limits: ResourceList{
				"cpu":            resource.MustParse("2"),
				"memory":         resource.MustParse("1Gi"),
				"nvidia.com/gpu": resource.MustParse("1"),
				"amd.com/gpu":    resource.MustParse("1"),
			},
			Requests: ResourceList{},
		},
		PersistentVolume: true,
		Volumes: []VolumeMount{
			{
				Name:      dev.GetVolumeName(),
				MountPath: OktetoSyncthingMountPath,
				SubPath:   SyncthingSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: RemoteMountPath,
				SubPath:   RemoteSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/app",
				SubPath:   SourceCodeSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/path",
				SubPath:   path.Join(SourceCodeSubPath, "sub"),
			},
		},
		InitContainer: InitContainer{Image: OktetoBinImageTag},
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
	}

	marshalled1, _ := yaml.Marshal(rule1)
	marshalled1OK, _ := yaml.Marshal(rule1OK)
	if string(marshalled1) != string(marshalled1OK) {
		t.Fatalf("Wrong rule1 generation.\nActual %s, \nExpected %s", string(marshalled1), string(marshalled1OK))
	}

	dev2 := dev.Services[0]
	rule2 := dev2.ToTranslationRule(dev, false)
	rule2OK := &TranslationRule{
		Container:       "dev",
		Image:           "worker:latest",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         nil,
		Args:            nil,
		Healthchecks:    true,
		Probes:          &Probes{Readiness: true, Liveness: true, Startup: true},
		Lifecycle:       &Lifecycle{PostStart: true, PostStop: false},
		Environment:     make(Environment, 0),
		SecurityContext: &SecurityContext{
			RunAsUser:  &rootUser,
			RunAsGroup: &rootUser,
			FSGroup:    &rootUser,
		},
		Resources:        ResourceRequirements{},
		PersistentVolume: true,
		Volumes: []VolumeMount{
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/src",
				SubPath:   path.Join(SourceCodeSubPath, "worker"),
			},
		},
	}

	marshalled2, _ := yaml.Marshal(rule2)
	marshalled2OK, _ := yaml.Marshal(rule2OK)
	if string(marshalled2) != string(marshalled2OK) {
		t.Fatalf("Wrong rule2 generation.\nActual %s, \nExpected %s", string(marshalled2), string(marshalled2OK))
	}
}

func TestDevToTranslationRuleInitContainer(t *testing.T) {
	manifest := []byte(`name: web
namespace: n
sync:
  - .:/app
initContainer:
  image: image
  resources:
    requests:
      cpu: 1
      memory: 1Gi
    limits:
      cpu: 2
      memory: 2Gi`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	rule := dev.ToTranslationRule(dev, false)
	ruleOK := &TranslationRule{
		Marker:            OktetoBinImageTag,
		OktetoBinImageTag: "image",
		ImagePullPolicy:   apiv1.PullAlways,
		Command:           []string{"/var/okteto/bin/start.sh"},
		Args:              []string{"-r"},
		Probes:            &Probes{},
		Lifecycle:         &Lifecycle{},
		Environment: Environment{
			{
				Name:  "OKTETO_NAMESPACE",
				Value: "n",
			},
			{
				Name:  "OKTETO_NAME",
				Value: "web",
			},
		},
		SecurityContext: &SecurityContext{
			RunAsUser:  &rootUser,
			RunAsGroup: &rootUser,
			FSGroup:    &rootUser,
		},
		Resources:        ResourceRequirements{},
		PersistentVolume: true,
		Volumes: []VolumeMount{
			{
				Name:      dev.GetVolumeName(),
				MountPath: OktetoSyncthingMountPath,
				SubPath:   SyncthingSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: RemoteMountPath,
				SubPath:   RemoteSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/app",
				SubPath:   SourceCodeSubPath,
			},
		},
		InitContainer: InitContainer{
			Image: "image",
			Resources: ResourceRequirements{
				Requests: map[apiv1.ResourceName]resource.Quantity{
					apiv1.ResourceCPU:    resource.MustParse("1"),
					apiv1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Limits: map[apiv1.ResourceName]resource.Quantity{
					apiv1.ResourceCPU:    resource.MustParse("2"),
					apiv1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
		},
	}

	marshalled, _ := yaml.Marshal(rule)
	marshalledOK, _ := yaml.Marshal(ruleOK)
	if string(marshalled) != string(marshalledOK) {
		t.Fatalf("Wrong rule generation.\nActual %s, \nExpected %s", string(marshalled), string(marshalledOK))
	}
}

func TestDevToTranslationRuleDockerEnabled(t *testing.T) {
	manifest := []byte(`name: web
image: dev-image
namespace: n
sync:
  - .:/app
docker:
  enabled: true`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	dev.Username = "cindy"
	dev.RegistryURL = "registry.okteto.dev"

	rule := dev.ToTranslationRule(dev, false)
	ruleOK := &TranslationRule{
		Marker:            OktetoBinImageTag,
		OktetoBinImageTag: OktetoBinImageTag,
		ImagePullPolicy:   apiv1.PullAlways,
		Image:             "dev-image",
		Command:           []string{"/var/okteto/bin/start.sh"},
		Args:              []string{"-r", "-d"},
		Probes:            &Probes{},
		Lifecycle:         &Lifecycle{},
		Environment: Environment{
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
				Value: DefaultDockerHost,
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
		SecurityContext: &SecurityContext{
			RunAsUser:  &rootUser,
			RunAsGroup: &rootUser,
			FSGroup:    &rootUser,
		},
		PersistentVolume: true,
		Volumes: []VolumeMount{
			{
				Name:      dev.GetVolumeName(),
				MountPath: DefaultDockerCertDir,
				SubPath:   DefaultDockerCertDirSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: DefaultDockerCacheDir,
				SubPath:   DefaultDockerCacheDirSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: OktetoSyncthingMountPath,
				SubPath:   SyncthingSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: RemoteMountPath,
				SubPath:   RemoteSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/app",
				SubPath:   SourceCodeSubPath,
			},
		},
		InitContainer: InitContainer{Image: OktetoBinImageTag},
		Docker: DinDContainer{
			Enabled: true,
			Image:   DefaultDinDImage,
		},
	}

	marshalled, _ := yaml.Marshal(rule)
	marshalledOK, _ := yaml.Marshal(ruleOK)
	if string(marshalled) != string(marshalledOK) {
		t.Fatalf("Wrong rule generation.\nActual %s, \nExpected %s", string(marshalled), string(marshalledOK))
	}
}

func TestDevToTranslationDebugEnabled(t *testing.T) {
	log.SetLevel("debug")
	defer log.SetLevel("info")
	manifest := []byte(`name: web
image: dev-image
namespace: n
sync:
  - .:/app`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	rule := dev.ToTranslationRule(dev, false)
	ruleOK := &TranslationRule{
		Marker:            OktetoBinImageTag,
		OktetoBinImageTag: OktetoBinImageTag,
		ImagePullPolicy:   apiv1.PullAlways,
		Image:             "dev-image",
		Command:           []string{"/var/okteto/bin/start.sh"},
		Args:              []string{"-r", "-v"},
		Probes:            &Probes{},
		Lifecycle:         &Lifecycle{},
		Environment: Environment{
			{
				Name:  "OKTETO_NAMESPACE",
				Value: "n",
			},
			{
				Name:  "OKTETO_NAME",
				Value: "web",
			},
		},
		SecurityContext: &SecurityContext{
			RunAsUser:  &rootUser,
			RunAsGroup: &rootUser,
			FSGroup:    &rootUser,
		},
		PersistentVolume: true,
		Volumes: []VolumeMount{
			{
				Name:      dev.GetVolumeName(),
				MountPath: OktetoSyncthingMountPath,
				SubPath:   SyncthingSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: RemoteMountPath,
				SubPath:   RemoteSubPath,
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/app",
				SubPath:   SourceCodeSubPath,
			},
		},
		InitContainer: InitContainer{Image: OktetoBinImageTag},
	}

	marshalled, _ := yaml.Marshal(rule)
	marshalledOK, _ := yaml.Marshal(ruleOK)
	if string(marshalled) != string(marshalledOK) {
		t.Fatalf("Wrong rule generation.\nActual %s, \nExpected %s", string(marshalled), string(marshalledOK))
	}
}

func TestSSHServerPortTranslationRule(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Dev
		expected Environment
	}{
		{
			name: "default",
			manifest: &Dev{
				Image:         &BuildInfo{},
				SSHServerPort: oktetoDefaultSSHServerPort,
			},
			expected: Environment{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
			},
		},
		{
			name: "custom port",
			manifest: &Dev{
				Image:         &BuildInfo{},
				SSHServerPort: 22220,
			},
			expected: Environment{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
				{Name: oktetoSSHServerPortVariable, Value: "22220"},
			},
		},
	}
	for _, test := range tests {
		t.Logf("test: %s", test.name)
		rule := test.manifest.ToTranslationRule(test.manifest, false)
		if e, a := test.expected, rule.Environment; !reflect.DeepEqual(e, a) {
			t.Errorf("expected environment:\n%#v\ngot:\n%#v", e, a)
		}
	}
}

func TestDevToTranslationRuleRunAsNonRoot(t *testing.T) {
	var falseBoolean = false
	var trueBoolean = true
	var runAsUser int64 = 100
	var runAsGroup int64 = 101
	var fsGroup int64 = 102

	tests := []struct {
		manifest   []byte
		translated SecurityContext
	}{
		{
			manifest: []byte(`name: non-root-user-without-overrides
image: worker:latest
namespace: n
securityContext:
   runAsNonRoot: true`),
			translated: SecurityContext{
				RunAsNonRoot: &trueBoolean,
			},
		},
		{
			manifest: []byte(`name: root-user-with-defaults
image: worker:latest
namespace: n
securityContext:
   runAsNonRoot: false`),
			translated: SecurityContext{
				RunAsUser:    &rootUser,
				RunAsGroup:   &rootUser,
				FSGroup:      &rootUser,
				RunAsNonRoot: &falseBoolean,
			},
		},
		{
			manifest: []byte(`name: non-root-user-with-overrides
image: worker:latest
namespace: n
securityContext:
   runAsUser: 100
   runAsGroup: 101
   fsGroup: 102
   runAsNonRoot: true`),
			translated: SecurityContext{
				RunAsUser:    &runAsUser,
				RunAsGroup:   &runAsGroup,
				FSGroup:      &fsGroup,
				RunAsNonRoot: &trueBoolean,
			},
		},
		{
			manifest: []byte(`name: root-user-with-overrides
image: worker:latest
namespace: n
securityContext:
   runAsUser: 100
   runAsGroup: 101
   fsGroup: 102
   runAsNonRoot: false`),
			translated: SecurityContext{
				RunAsUser:    &runAsUser,
				RunAsGroup:   &runAsGroup,
				FSGroup:      &fsGroup,
				RunAsNonRoot: &falseBoolean,
			},
		},
		{
			manifest: []byte(`name: no-security-context
image: worker:latest
namespace: n`),
			translated: SecurityContext{
				RunAsUser:  &rootUser,
				RunAsGroup: &rootUser,
				FSGroup:    &rootUser,
			},
		},
		{
			manifest: []byte(`name: no-run-as-non-root
image: worker:latest
namespace: n
securityContext:
   runAsUser: 100
   runAsGroup: 101
   fsGroup: 102`),
			translated: SecurityContext{
				RunAsUser:  &runAsUser,
				RunAsGroup: &runAsGroup,
				FSGroup:    &fsGroup,
			},
		},
	}

	for _, test := range tests {
		dev, err := Read(test.manifest)
		if err != nil {
			t.Fatal(err)
		}
		rule := dev.ToTranslationRule(dev, false)
		marshalled, _ := yaml.Marshal(rule.SecurityContext)
		marshalledOK, _ := yaml.Marshal(test.translated)
		if string(marshalled) != string(marshalledOK) {
			t.Fatalf("Wrong rule generation for %s.\nActual %s, \nExpected %s", dev.Name, string(marshalled), string(marshalledOK))
		}
	}

}
