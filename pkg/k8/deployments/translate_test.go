package deployments

import (
	"testing"

	"github.com/cloudnativedevelopment/cnd/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
	updateCndContainer(c, dev, "mynamespace")

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

	if c.Env == nil || c.Env[0].Name != cndEnvNamespace {
		t.Errorf("CND dev context wasn't set: %+v", c)
	}

}

func Test_mergeEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name       string
		deployment []v1.EnvVar
		dev        map[string]string
		expected   []v1.EnvVar
	}{
		{
			"both-nil",
			nil,
			nil,
			[]v1.EnvVar{
				v1.EnvVar{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
			},
		},
		{
			"both-empty",
			[]v1.EnvVar{},
			map[string]string{},
			[]v1.EnvVar{
				v1.EnvVar{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
			},
		},
		{
			"no-overlap",
			[]v1.EnvVar{
				v1.EnvVar{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				v1.EnvVar{
					Name:  "another-deployment",
					Value: "another-value-from-deployment",
				},
			},
			map[string]string{
				"dev":  "on",
				"test": "true",
			},
			[]v1.EnvVar{
				v1.EnvVar{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
				v1.EnvVar{
					Name:  "another-deployment",
					Value: "another-value-from-deployment",
				},
				v1.EnvVar{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				v1.EnvVar{
					Name:  "dev",
					Value: "on",
				},
				v1.EnvVar{
					Name:  "test",
					Value: "true",
				},
			},
		},
		{
			"overlap",
			[]v1.EnvVar{
				v1.EnvVar{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				v1.EnvVar{
					Name:  "another-deployment",
					Value: "another-value-from-deployment",
				},
			},
			map[string]string{
				"dev":                "on",
				"test":               "true",
				"another-deployment": "overriden-value-from-dev",
			},
			[]v1.EnvVar{
				v1.EnvVar{
					Name:  "CND_KUBERNETES_NAMESPACE",
					Value: "cnd-namespace",
				},
				v1.EnvVar{
					Name:  "another-deployment",
					Value: "overriden-value-from-dev",
				},
				v1.EnvVar{
					Name:  "deployment",
					Value: "value-from-deployment",
				},
				v1.EnvVar{
					Name:  "dev",
					Value: "on",
				},
				v1.EnvVar{
					Name:  "test",
					Value: "true",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEnvironmentVariables(tt.deployment, tt.dev, "cnd-namespace")
			for i := range result {
				if result[i].Name != tt.expected[i].Name {
					t.Fatalf("failed to merge name. '%s' != '%s'.\nActual\n%+v\nExpected\n%+v", result[i].Name, tt.expected[i].Name, result, tt.expected)
				}

				if result[i].Value != tt.expected[i].Value {
					t.Fatalf("failed to merge value. '%s' != '%s'.\nActual\n%+v\nExpected\n%+v", result[i].Value, tt.expected[i].Value, result, tt.expected)
				}
			}

		})
	}
}
