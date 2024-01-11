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

package model

import (
	"bytes"
	"path"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestDevToTranslationRule(t *testing.T) {
	manifestBytes := []byte(`name: web
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
      - worker:/src`)

	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	dev := manifest.Dev["web"]

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
		Environment: env.Environment{
			{
				Name:  "OKTETO_NAMESPACE",
				Value: "n",
			},
			{
				Name:  "OKTETO_NAME",
				Value: "web",
			},
			{Name: "HISTSIZE", Value: "10000000"},
			{Name: "HISTFILESIZE", Value: "10000000"},
			{Name: "HISTCONTROL", Value: "ignoreboth:erasedups"},
			{Name: "HISTFILE", Value: "/var/okteto/bashrc/.bash_history"},
			{Name: "BASHOPTS", Value: "histappend"},
			{Name: "PROMPT_COMMAND", Value: "history -a ; history -c ; history -r"},
		},
		SecurityContext: &SecurityContext{
			RunAsUser:  pointer.Int64(0),
			RunAsGroup: pointer.Int64(0),
			FSGroup:    pointer.Int64(0),
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
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/var/okteto/bashrc",
				SubPath:   "okteto-bash-history",
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

	marshalled1, err := yaml.Marshal(rule1)
	assert.NoError(t, err)
	marshalled1OK, err := yaml.Marshal(rule1OK)
	assert.NoError(t, err)
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
		Healthchecks:    false,
		Probes:          &Probes{},
		Lifecycle:       &Lifecycle{},
		SecurityContext: &SecurityContext{
			RunAsUser:  pointer.Int64(0),
			RunAsGroup: pointer.Int64(0),
			FSGroup:    pointer.Int64(0),
		},
		Resources:        ResourceRequirements{},
		PersistentVolume: true,
		Environment: env.Environment{
			{Name: "HISTSIZE", Value: "10000000"},
			{Name: "HISTFILESIZE", Value: "10000000"},
			{Name: "HISTCONTROL", Value: "ignoreboth:erasedups"},
			{Name: "HISTFILE", Value: "/var/okteto/bashrc/.bash_history"},
			{Name: "BASHOPTS", Value: "histappend"},
			{Name: "PROMPT_COMMAND", Value: "history -a ; history -c ; history -r"},
		},
		Volumes: []VolumeMount{
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/src",
				SubPath:   path.Join(SourceCodeSubPath, "worker"),
			},
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/var/okteto/bashrc",
				SubPath:   "okteto-bash-history",
			},
		},
		Secrets: make([]Secret, 0),
	}

	marshalled2, err := yaml.Marshal(rule2)
	assert.NoError(t, err)
	marshalled2OK, err := yaml.Marshal(rule2OK)
	assert.NoError(t, err)

	if !assert.Equal(t, rule2, rule2OK) {
		t.Fatalf("Wrong rule2 generation.\nActual %s, \nExpected %s", string(marshalled2), string(marshalled2OK))
	}
}

func TestDevToTranslationRuleInitContainer(t *testing.T) {
	manifestBytes := []byte(`name: web
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

	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	dev := manifest.Dev["web"]

	rule := dev.ToTranslationRule(dev, false)
	ruleOK := &TranslationRule{
		Marker:            OktetoBinImageTag,
		OktetoBinImageTag: "image",
		ImagePullPolicy:   apiv1.PullAlways,
		Command:           []string{"/var/okteto/bin/start.sh"},
		Args:              []string{"-r"},
		Probes:            &Probes{},
		Lifecycle:         &Lifecycle{},
		Environment: env.Environment{
			{
				Name:  "OKTETO_NAMESPACE",
				Value: "n",
			},
			{
				Name:  "OKTETO_NAME",
				Value: "web",
			},
			{Name: "HISTSIZE", Value: "10000000"},
			{Name: "HISTFILESIZE", Value: "10000000"},
			{Name: "HISTCONTROL", Value: "ignoreboth:erasedups"},
			{Name: "HISTFILE", Value: "/var/okteto/bashrc/.bash_history"},
			{Name: "BASHOPTS", Value: "histappend"},
			{Name: "PROMPT_COMMAND", Value: "history -a ; history -c ; history -r"},
		},
		SecurityContext: &SecurityContext{
			RunAsUser:  pointer.Int64(0),
			RunAsGroup: pointer.Int64(0),
			FSGroup:    pointer.Int64(0),
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
			{
				Name:      dev.GetVolumeName(),
				MountPath: "/var/okteto/bashrc",
				SubPath:   "okteto-bash-history",
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

	marshalled, err := yaml.Marshal(rule)
	assert.NoError(t, err)
	marshalledOK, err := yaml.Marshal(ruleOK)
	assert.NoError(t, err)
	if !bytes.Equal(marshalled, marshalledOK) {
		t.Fatalf("Wrong rule generation.\nActual %s, \nExpected %s", string(marshalled), string(marshalledOK))
	}
}

func TestDevToTranslationDebugEnabled(t *testing.T) {
	oktetoLog.SetLevel("debug")
	defer oktetoLog.SetLevel(oktetoLog.InfoLevel)
	manifestBytes := []byte(`name: web
image: dev-image
namespace: n
sync:
  - .:/app`)

	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	dev := manifest.Dev["web"]

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
		Environment: env.Environment{
			{
				Name:  "OKTETO_NAMESPACE",
				Value: "n",
			},
			{
				Name:  "OKTETO_NAME",
				Value: "web",
			},
			{Name: "HISTSIZE", Value: "10000000"},
			{Name: "HISTFILESIZE", Value: "10000000"},
			{Name: "HISTCONTROL", Value: "ignoreboth:erasedups"},
			{Name: "HISTFILE", Value: "/var/okteto/bashrc/.bash_history"},
			{Name: "BASHOPTS", Value: "histappend"},
			{Name: "PROMPT_COMMAND", Value: "history -a ; history -c ; history -r"},
		},
		SecurityContext: &SecurityContext{
			RunAsUser:  pointer.Int64(0),
			RunAsGroup: pointer.Int64(0),
			FSGroup:    pointer.Int64(0),
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
				MountPath: "/var/okteto/bashrc",
				SubPath:   "okteto-bash-history",
			},
		},
		InitContainer: InitContainer{Image: OktetoBinImageTag},
	}

	marshalled, err := yaml.Marshal(rule)
	assert.NoError(t, err)
	marshalledOK, err := yaml.Marshal(ruleOK)
	assert.NoError(t, err)
	if !bytes.Equal(marshalled, marshalledOK) {
		t.Fatalf("Wrong rule generation.\nActual %s, \nExpected %s", string(marshalled), string(marshalledOK))
	}
}

func TestSSHServerPortTranslationRule(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Dev
		expected env.Environment
	}{
		{
			name: "default",
			manifest: &Dev{
				Image:         &build.Info{},
				SSHServerPort: oktetoDefaultSSHServerPort,
			},
			expected: env.Environment{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
				{Name: "HISTSIZE", Value: "10000000"},
				{Name: "HISTFILESIZE", Value: "10000000"},
				{Name: "HISTCONTROL", Value: "ignoreboth:erasedups"},
				{Name: "HISTFILE", Value: "/var/okteto/bashrc/.bash_history"},
				{Name: "BASHOPTS", Value: "histappend"},
				{Name: "PROMPT_COMMAND", Value: "history -a ; history -c ; history -r"},
			},
		},
		{
			name: "custom port",
			manifest: &Dev{
				Image:         &build.Info{},
				SSHServerPort: 22220,
			},
			expected: env.Environment{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
				{Name: oktetoSSHServerPortVariable, Value: "22220"},
				{Name: "HISTSIZE", Value: "10000000"},
				{Name: "HISTFILESIZE", Value: "10000000"},
				{Name: "HISTCONTROL", Value: "ignoreboth:erasedups"},
				{Name: "HISTFILE", Value: "/var/okteto/bashrc/.bash_history"},
				{Name: "BASHOPTS", Value: "histappend"},
				{Name: "PROMPT_COMMAND", Value: "history -a ; history -c ; history -r"},
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
		translated SecurityContext
		name       string
		manifest   []byte
	}{
		{
			name: "root-user-with-overrides",
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
			name: "non-root-user-without-overrides",
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
			name: "root-user-with-defaults",
			manifest: []byte(`name: root-user-with-defaults
image: worker:latest
namespace: n
securityContext:
   runAsNonRoot: false`),
			translated: SecurityContext{
				RunAsUser:    pointer.Int64(0),
				RunAsGroup:   pointer.Int64(0),
				FSGroup:      pointer.Int64(0),
				RunAsNonRoot: &falseBoolean,
			},
		},
		{
			name: "non-root-user-with-overrides",
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
			name: "no-security-context",
			manifest: []byte(`name: no-security-context
image: worker:latest
namespace: n`),
			translated: SecurityContext{
				RunAsUser:  pointer.Int64(0),
				RunAsGroup: pointer.Int64(0),
				FSGroup:    pointer.Int64(0),
			},
		},
		{
			name: "no-run-as-non-root",
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
		manifest, err := Read(test.manifest)
		if err != nil {
			t.Fatal(err)
		}

		dev := manifest.Dev[test.name]

		rule := dev.ToTranslationRule(dev, false)
		marshalled, err := yaml.Marshal(rule.SecurityContext)
		assert.NoError(t, err)
		marshalledOK, err := yaml.Marshal(test.translated)
		assert.NoError(t, err)
		if !bytes.Equal(marshalled, marshalledOK) {
			t.Fatalf("Wrong rule generation for %s.\nActual %s, \nExpected %s", dev.Name, string(marshalled), string(marshalledOK))
		}
	}

}
