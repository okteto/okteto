package deployments

import (
	"testing"

	"cli/cnd/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

func Test_updateCNDContainer(t *testing.T) {
	dev := &model.Dev{
		Name:      "deployment",
		Container: "api",
		Image:     "okteto/test",
		WorkDir: &model.Mount{
			Path: "/app",
		},
	}
	c := &apiv1.Container{
		Command: []string{"/run"},
		Args:    []string{"all"},
	}
	updateCndContainer(c, dev, "mynamespace")

	if c.Image != "okteto/test" {
		t.Errorf("Image wasn't updated: %+v", c)
	}

	if c.Command[0] != "tail" {
		t.Errorf("Command was updated: %+v", c)
	}

	if c.Args[0] != "-f" {
		t.Errorf("Args was updated: %+v", c)
	}

	if c.WorkingDir != "/app" {
		t.Errorf("WorkingDir wasn't updated: %+v", c)
	}

	if c.VolumeMounts[0].MountPath != c.WorkingDir {
		t.Errorf("CND mount wasn't set: %+v", c)
	}

	if c.Env == nil || c.Env[0].Name != cndEnvNamespace {
		t.Errorf("CND dev context wasn't set: %+v", c)
	}

}

func Test_mergeEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name      string
		container *v1.Container
		dev       []model.EnvVar
		expected  []v1.EnvVar
	}{
		{
			"both-nil",
			&v1.Container{Env: nil},
			nil,
			[]v1.EnvVar{
				{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
			},
		},
		{
			"both-empty",
			&v1.Container{Env: []v1.EnvVar{}},
			[]model.EnvVar{},
			[]v1.EnvVar{
				{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
			},
		},
		{
			"no-overlap",
			&v1.Container{Env: []v1.EnvVar{
				{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				{
					Name:  "another-deployment",
					Value: "another-value-from-deployment",
				},
			}},
			[]model.EnvVar{
				{
					Name:  "dev",
					Value: "on"},
				{
					Name:  "test",
					Value: "true",
				},
			},
			[]v1.EnvVar{
				{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				{
					Name:  "another-deployment",
					Value: "another-value-from-deployment",
				},
				{
					Name:  "dev",
					Value: "on",
				},
				{
					Name:  "test",
					Value: "true",
				},
				{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
			},
		},
		{
			"overlap",
			&v1.Container{Env: []v1.EnvVar{
				{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				{
					Name:  "another-deployment",
					Value: "another-value-from-deployment",
				},
			}},
			[]model.EnvVar{
				{
					Name:  "dev",
					Value: "on",
				},
				{
					Name:  "test",
					Value: "true",
				},
				{
					Name:  "another-deployment",
					Value: "overriden-value-from-dev",
				},
			},
			[]v1.EnvVar{
				{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				{
					Name:  "another-deployment",
					Value: "overriden-value-from-dev",
				},
				{
					Name:  "dev",
					Value: "on",
				},
				{
					Name:  "test",
					Value: "true",
				},
				{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeEnvironmentVariables(tt.container, tt.dev, "cnd-namespace", false)
			for i, envvar := range tt.container.Env {
				if envvar.Name != tt.expected[i].Name {
					t.Fatalf("failed to merge name. '%s' != '%s'.\nActual\n%+v\nExpected\n%+v", envvar.Name, tt.expected[i].Name, tt.container.Env, tt.expected)
				}

				if envvar.Value != tt.expected[i].Value {
					t.Fatalf("failed to merge value. '%s' != '%s'.\nActual\n%+v\nExpected\n%+v", envvar.Value, tt.expected[i].Value, tt.container.Env, tt.expected)
				}
			}

		})
	}
}
