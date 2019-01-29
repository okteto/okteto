package model

import (
	"fmt"
	"os"
	"reflect"
	"testing"
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
		name     string
		manifest []byte
		expected []string
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
			[]string{"--gevent", "100", "--http-socket", "0.0.0.0:8000", "--mount", "/=app:app", "--python-autoreload", "1"},
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
			[]string{"start.sh"},
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
			if reflect.DeepEqual(d.Scripts["run"], tt.expected) {
				t.Errorf("script was not parsed correctly")
			}

		})
	}

}
