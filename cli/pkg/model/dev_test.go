package model

import (
	"reflect"
	"testing"
)

func Test_oldLoadDev(t *testing.T) {
	manifest := []byte(`
swap:
  deployment:
    name: deployment
    container: core
    image: code/core:0.1.8
    command: ["uwsgi"]
    args: ["--gevent", "100", "--http-socket", "0.0.0.0:8000", "--mount", "/=codescope:app", "--python-autoreload", "1"]
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
mount:
  target: /app`)
	d, err := read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if d.Name != "deployment" {
		t.Errorf("name was not parsed: %+v", d)
	}

	if d.WorkDir.Path != "/app" {
		t.Errorf("workdir.path was not parsed: %+v", d)
	}

	if len(d.Command) != 1 || d.Command[0] != "uwsgi" {
		t.Errorf("command was not parsed: %+v", d)
	}

	if len(d.Args) != 8 || d.Args[4] != "--mount" {
		t.Errorf("args was not parsed: %+v", d)
	}

	memory := d.Resources.Requests["memory"]
	if memory.String() != "64Mi" {
		t.Errorf("Resources.Requests.Memory was not parsed: %s", memory.String())
	}

	cpu := d.Resources.Requests["cpu"]
	if cpu.String() != "250m" {
		t.Errorf("Resources.Requests.CPU was not parsed correctly. Expected '250M', got '%s'", cpu.String())
	}

	memory = d.Resources.Limits["memory"]
	if memory.String() != "128Mi" {
		t.Errorf("Resources.Requests.Memory was not parsed: %s", memory.String())
	}

	cpu = d.Resources.Limits["cpu"]
	if cpu.String() != "500m" {
		t.Errorf("Resources.Requests.CPU was not parsed correctly. Expected '500M', got '%s'", cpu.String())
	}
}

func Test_loadDev(t *testing.T) {
	manifest := []byte(`
name: deployment
container: core
image: code/core:0.1.8
command: ["uwsgi"]
args: ["--gevent", "100", "--http-socket", "0.0.0.0:8000", "--mount", "/=codescope:app", "--python-autoreload", "1"]
resources:
  requests:
    memory: "64Mi"
    cpu: "250m"
  limits:
    memory: "128Mi"
    cpu: "500m"
workdir: /app`)
	d, err := read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if d.Name != "deployment" {
		t.Errorf("name was not parsed: %+v", d)
	}

	if len(d.Command) != 1 || d.Command[0] != "uwsgi" {
		t.Errorf("command was not parsed: %+v", d)
	}

	if len(d.Args) != 8 || d.Args[4] != "--mount" {
		t.Errorf("args was not parsed: %+v", d)
	}

	memory := d.Resources.Requests["memory"]
	if memory.String() != "64Mi" {
		t.Errorf("Resources.Requests.Memory was not parsed: %s", memory.String())
	}

	cpu := d.Resources.Requests["cpu"]
	if cpu.String() != "250m" {
		t.Errorf("Resources.Requests.CPU was not parsed correctly. Expected '250M', got '%s'", cpu.String())
	}

	memory = d.Resources.Limits["memory"]
	if memory.String() != "128Mi" {
		t.Errorf("Resources.Requests.Memory was not parsed: %s", memory.String())
	}

	cpu = d.Resources.Limits["cpu"]
	if cpu.String() != "500m" {
		t.Errorf("Resources.Requests.CPU was not parsed correctly. Expected '500M', got '%s'", cpu.String())
	}
}

func Test_loadDevDefaults(t *testing.T) {
	var tests = []struct {
		name                string
		manifest            []byte
		expectedScripts     map[string]string
		expectedEnvironment []EnvVar
		expectedForward     []Forward
	}{
		{
			"long script",
			[]byte(`
            name: service
            container: core
            workdir:
              path: /app
            scripts:
              run: "uwsgi --gevent 100 --http-socket 0.0.0.0:8000 --mount /=app:app --python-autoreload 1"`),
			map[string]string{
				"run": "uwsgi --gevent 100 --http-socket 0.0.0.0:8000 --mount /=app:app --python-autoreload 1"},
			[]EnvVar{},
			[]Forward{},
		},
		{
			"basic script",
			[]byte(`
            name: service
            container: core
            workdir: /app
            scripts:
              run: "start.sh"`),
			map[string]string{"run": "start.sh"},
			[]EnvVar{},
			[]Forward{},
		},
		{
			"env vars",
			[]byte(`
            name: service
            container: core
            workdir:
              path: /app
            scripts:
              run: "start.sh"
            environment:
                - ENV=production
                - name=test-node`),
			map[string]string{"run": "start.sh"},
			[]EnvVar{
				{Name: "ENV", Value: "production"},
				{Name: "name", Value: "test-node"},
			},
			[]Forward{},
		},
		{
			"forward",
			[]byte(`
            name: service
            container: core
            workdir: /app
            scripts:
              run: "start.sh"
            forward:
                - 9000:8000
                - 9001:8001`),
			map[string]string{"run": "start.sh"},
			[]EnvVar{},
			[]Forward{
				{Local: 9000, Remote: 8000},
				{Local: 9001, Remote: 8001},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			if len(d.Command) != 1 || d.Command[0] != "sh" {
				t.Errorf("command was parsed: %+v", d)
			}

			if len(d.Args) != 0 {
				t.Errorf("args was parsed: %+v", d)
			}

			if !reflect.DeepEqual(d.Environment, tt.expectedEnvironment) {
				t.Errorf("environment was not parsed correctly:\n%+v\n%+v", d.Environment, tt.expectedEnvironment)
			}

			if !reflect.DeepEqual(d.Forward, tt.expectedForward) {
				t.Errorf("environment was not parsed correctly:\n%+v\n%+v", d.Forward, tt.expectedForward)
			}

			if !reflect.DeepEqual(d.Scripts, tt.expectedScripts) {
				t.Errorf("script was not parsed correctly:\n%+v\n%+v", d.Scripts, tt.expectedScripts)
			}

			for k, v := range d.Resources.Limits {
				if v.IsZero() {
					t.Errorf("resources.limits.%s wasn't set", k)
				}
			}

			for k, v := range d.Resources.Requests {
				if !v.IsZero() {
					t.Errorf("resources.limits.%s was set", k)
				}
			}

		})
	}
}
