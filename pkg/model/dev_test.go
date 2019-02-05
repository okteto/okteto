package model

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func Test_fixPath(t *testing.T) {
	wd, _ := os.Getwd()

	var tests = []struct {
		name     string
		source   string
		target   string
		devPath  string
		expected string
	}{
		{
			name:     "relative-source",
			source:   ".",
			target:   "/go/src/github.com/cloudnativedevelopment/cnd",
			devPath:  "/go/src/github.com/cloudnativedevelopment/cnd/cnd.yml",
			expected: "/go/src/github.com/cloudnativedevelopment/cnd"},
		{
			name:     "relative-source-abs",
			source:   "/go/src/github.com/cloudnativedevelopment/cnd",
			target:   "/src/github.com/cloudnativedevelopment/cnd",
			devPath:  "cnd.yml",
			expected: "/go/src/github.com/cloudnativedevelopment/cnd"},
		{
			name:     "relative-dev-path",
			source:   "k8/src",
			target:   "/go/src/github.com/cloudnativedevelopment/cnd",
			devPath:  "cnd/cnd.yml",
			expected: fmt.Sprintf("%s/cnd/k8/src", wd),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := Dev{
				Mount: Mount{
					Source: tt.source,
					Target: tt.target,
				},
			}

			dev.fixPath(tt.devPath)
			if dev.Mount.Source != tt.expected {
				t.Errorf("%s != %s", dev.Mount.Source, tt.expected)
			}
		})
	}
}

func Test_loadDev(t *testing.T) {
	manifest := []byte(`
swap:
  deployment:
    name: deployment
    container: core
    image: codescope/core:0.1.8
    command: ["uwsgi"]
    args: ["--gevent", "100", "--http-socket", "0.0.0.0:8000", "--mount", "/=codescope:app", "--python-autoreload", "1"]
mount:
  source: /Users/example/app
  target: /app`)
	d, err := LoadDev(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if d.Swap.Deployment.Name != "deployment" {
		t.Errorf("name was not parsed: %+v", d)
	}

	if len(d.Swap.Deployment.Command) != 1 || d.Swap.Deployment.Command[0] != "uwsgi" {
		t.Errorf("command was not parsed: %+v", d)
	}

	if len(d.Swap.Deployment.Args) != 8 || d.Swap.Deployment.Args[4] != "--mount" {
		t.Errorf("args was not parsed: %+v", d)
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
            swap:
              deployment:
                name: service
                container: core
            mount:
              source: /Users/example/app
              target: /app
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
            swap:
              deployment:
                name: service
                container: core
            mount:
              source: /src
              target: /app
            scripts:
              run: "start.sh"`),
			map[string]string{"run": "start.sh"},
			[]EnvVar{},
			[]Forward{},
		},
		{
			"env vars",
			[]byte(`
            swap:
              deployment:
                name: service
                container: core
            mount:
              source: /src
              target: /app
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
            swap:
              deployment:
                name: service
                container: core
            mount:
              source: /src
              target: /app
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
			d, err := LoadDev(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			if d.Swap.Deployment.Command != nil || len(d.Swap.Deployment.Command) != 0 {
				t.Errorf("command was not parsed: %+v", d)
			}

			if d.Swap.Deployment.Args != nil || len(d.Swap.Deployment.Args) != 0 {
				t.Errorf("args was not parsed: %+v", d)
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

		})
	}

}
func TestEnvVarMashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected EnvVar
	}{
		{
			"key-value",
			[]byte(`env=production`),
			EnvVar{Name: "env", Value: "production"},
		},
		{
			"key-value-complex",
			[]byte(`env='production=11231231asa#$˜GADAFA'`),
			EnvVar{Name: "env", Value: "'production=11231231asa#$˜GADAFA'"},
		},
		{
			"key-value-os",
			[]byte(`env=$DEV_ENV`),
			EnvVar{Name: "env", Value: "test_environment"},
		},
		{
			"no-value-no-os",
			[]byte(`noenv`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"no-value-no-os-equal",
			[]byte(`noenv=`),
			EnvVar{Name: "noenv", Value: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result EnvVar
			if err := os.Setenv("DEV_ENV", "test_environment"); err != nil {
				t.Fatal(err)
			}

			if err := yaml.Unmarshal(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}

			_, err := yaml.Marshal(&result)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
