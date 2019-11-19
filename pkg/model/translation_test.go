package model

import (
	"testing"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

func TestDevToTranslationRule(t *testing.T) {
	manifest := []byte(`name: web
container: dev
image: web:latest
command: ["./run_web.sh"]
imagePullPolicy: Never
volumes:
  - sub:/path
mountpath: /app
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
    mountpath: /src
    subpath: /worker`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	d1 := dev.GevSandbox()
	dev.DevPath = "okteto.yml"
	rule1 := dev.ToTranslationRule(dev, d1)
	rule1OK := &TranslationRule{
		Marker:          "okteto.yml",
		Container:       "dev",
		Image:           "web:latest",
		ImagePullPolicy: apiv1.PullNever,
		Command:         []string{"/var/okteto/bin/start.sh"},
		Args:            []string{},
		Healthchecks:    false,
		Environment: []EnvVar{
			{
				Name:  oktetoMarkerPathVariable,
				Value: "/app/okteto.yml",
			},
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
		Volumes: []VolumeMount{
			VolumeMount{
				Name:      OktetoVolumeName,
				MountPath: "/app",
				SubPath:   "web/data-0",
			},
			{
				Name:      OktetoVolumeName,
				MountPath: oktetoSyncthingMountPath,
				SubPath:   dev.syncthingSubPath(),
			},
			VolumeMount{
				Name:      OktetoVolumeName,
				MountPath: "/path",
				SubPath:   "web/data-0/sub",
			},
		},
	}

	marshalled1, _ := yaml.Marshal(rule1)
	marshalled1OK, _ := yaml.Marshal(rule1OK)
	if string(marshalled1) != string(marshalled1OK) {
		t.Fatalf("Wrong rule1 generation.\nActual %s, \nExpected %s", string(marshalled1), string(marshalled1OK))
	}

	dev2 := dev.Services[0]
	d2 := dev2.GevSandbox()
	rule2 := dev2.ToTranslationRule(dev, d2)
	rule2OK := &TranslationRule{
		Container:       "dev",
		Image:           "worker:latest",
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Command:         nil,
		Args:            nil,
		Healthchecks:    true,
		Environment:     make([]EnvVar, 0),
		Resources: ResourceRequirements{
			Limits:   ResourceList{},
			Requests: ResourceList{},
		},
		Volumes: []VolumeMount{
			{
				Name:      OktetoVolumeName,
				MountPath: "/src",
				SubPath:   "web/data-0/worker",
			},
		},
	}

	marshalled2, _ := yaml.Marshal(rule2)
	marshalled2OK, _ := yaml.Marshal(rule2OK)
	if string(marshalled2) != string(marshalled2OK) {
		t.Fatalf("Wrong rule2 generation.\nActual %s, \nExpected %s", string(marshalled2), string(marshalled2OK))
	}
}
