package model

import (
	"reflect"
	"testing"
)

func TestDevToTranslationRule(t *testing.T) {
	manifest := []byte(`name: web
container: dev
image: web:latest
command: ["./run_web.sh"]
mountpath: /app
services:
  - name: worker
    container: dev
    image: worker:latest
    mountpath: /src`)

	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	d1 := dev.GevSandbox()
	rule1 := dev.ToTranslationRule(dev, d1, "node")
	rule1OK := &TranslationRule{
		Node:        "node",
		Container:   "dev",
		Image:       "web:latest",
		Command:     []string{"tail"},
		Args:        []string{"-f", "/dev/null"},
		Environment: dev.Environment,
		Resources:   dev.Resources,
		Volumes: []VolumeMount{
			VolumeMount{
				Name:      "pvc-0-okteto-web-0",
				MountPath: "/app",
			},
		},
	}

	if !reflect.DeepEqual(rule1, rule1OK) {
		t.Fatalf("Wrong rule1 generation. Actual %+v, Expected %+v", rule1, rule1OK)
	}

	dev2 := dev.Services[0]
	d2 := dev2.GevSandbox()
	rule2 := dev2.ToTranslationRule(dev, d2, "node")
	rule2OK := &TranslationRule{
		Node:        "node",
		Container:   "dev",
		Image:       "worker:latest",
		Command:     nil,
		Args:        nil,
		Environment: dev2.Environment,
		Resources:   dev2.Resources,
		Volumes: []VolumeMount{
			VolumeMount{
				Name:      "pvc-0-okteto-web-0",
				MountPath: "/src",
			},
		},
	}

	if !reflect.DeepEqual(rule2, rule2OK) {
		t.Fatalf("Wrong rule2 generation. Actual %+v, Expected %+v", rule2, rule2OK)
	}
}
