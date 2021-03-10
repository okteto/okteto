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

package model

import (
	"path"
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
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
services:
  - name: worker
    container: dev
    image: worker:latest
    imagePullPolicy: IfNotPresent
    sync:
      - worker:/src
    healthchecks: 
      readiness: true
      liveness: true`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	rule1 := dev.ToTranslationRule(dev)
	rule1OK := &TranslationRule{
		Marker:            OktetoBinImageTag,
		OktetoBinImageTag: OktetoBinImageTag,
		Container:         "dev",
		Image:             "web:latest",
		ImagePullPolicy:   apiv1.PullNever,
		Command:           []string{"/var/okteto/bin/start.sh"},
		Args:              []string{"-r"},
		Healthchecks:      &HealthchecksProbes{},
		Environment: []EnvVar{
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
		InitContainer: InitContainer{
			Image: OktetoBinImageTag,
		},
	}

	marshalled1, _ := yaml.Marshal(rule1)
	marshalled1OK, _ := yaml.Marshal(rule1OK)
	if string(marshalled1) != string(marshalled1OK) {
		t.Fatalf("Wrong rule1 generation.\nActual %s, \nExpected %s", string(marshalled1), string(marshalled1OK))
	}

	dev2 := dev.Services[0]
	rule2 := dev2.ToTranslationRule(dev)
	rule2OK := &TranslationRule{
		Container:       "dev",
		Image:           "worker:latest",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         nil,
		Args:            nil,
		Healthchecks:    &HealthchecksProbes{Readiness: true, Liveness: true},
		Environment:     make([]EnvVar, 0),
		SecurityContext: &SecurityContext{
			RunAsUser:  &rootUser,
			RunAsGroup: &rootUser,
			FSGroup:    &rootUser,
		},
		Resources: ResourceRequirements{
			Limits:   ResourceList{},
			Requests: ResourceList{},
		},
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

func TestSSHServerPortTranslationRule(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Dev
		expected []EnvVar
	}{
		{
			name: "default",
			manifest: &Dev{
				Image:         &BuildInfo{},
				SSHServerPort: oktetoDefaultSSHServerPort,
			},
			expected: []EnvVar{
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
			expected: []EnvVar{
				{Name: "OKTETO_NAMESPACE", Value: ""},
				{Name: "OKTETO_NAME", Value: ""},
				{Name: oktetoSSHServerPortVariable, Value: "22220"},
			},
		},
	}
	for _, test := range tests {
		t.Logf("test: %s", test.name)
		rule := test.manifest.ToTranslationRule(test.manifest)
		if e, a := test.expected, rule.Environment; !reflect.DeepEqual(e, a) {
			t.Errorf("expected environment:\n%#v\ngot:\n%#v", e, a)
		}
	}
}
