package deployments

import (
	"testing"

	"github.com/okteto/cnd/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

func Test_updateCNDContainer(t *testing.T) {
	dev := &model.Dev{
		Swap: model.Swap{
			Deployment: model.Deployment{
				Name:      "deployment",
				Container: "api",
				Image:     "okteto/test",
			},
		},
		Mount: model.Mount{
			Source: ".",
			Target: "/app",
		},
	}
	c := &apiv1.Container{
		Command: []string{"/run"},
		Args:    []string{"all"},
	}
	updateCndContainer(c, dev)

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
