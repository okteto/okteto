package model

import (
	"testing"

	apiv1 "k8s.io/api/core/v1"
)

func Test_updateCNDContainer(t *testing.T) {
	manifest := []byte(`
swap:
  deployment:
    name: deployment
    container: core
    image: okteto/test
mount:
  source: /Users/fernandomayofernandez/PycharmProjects/codescope-core
  target: /app`)
	d, err := loadDev(manifest)
	if err != nil {
		t.Fatal(err)
	}

	c := &apiv1.Container{
		Command: []string{"/run"},
		Args:    []string{"all"},
	}
	d.updateCndContainer(c)

	if c.Image != "okteto/test" {
		t.Errorf("Image wasn't updated: %+v", c)
	}

	if c.Command[0] != "/run" {
		t.Errorf("Command was updated: %+v", c)
	}

	if c.Args[0] != "all" {
		t.Errorf("Args was updated: %+v", c)
	}

	if c.WorkingDir != "/app" {
		t.Errorf("WorkingDir wasn't updated: %+v", c)
	}

	if c.VolumeMounts[0].MountPath != c.WorkingDir {
		t.Errorf("CND mount wasn't set: %+v", c)
	}

}
