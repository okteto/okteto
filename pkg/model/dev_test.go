package model

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	apiv1 "k8s.io/api/core/v1"
)

func Test_loadDev(t *testing.T) {
	manifest := []byte(`
name: deployment
container: core
image: code/core:0.1.8
command: ["uwsgi"]
annotations:
  key1: value1
  key2: value2
resources:
  requests:
    memory: "64Mi"
    cpu: "250m"
  limits:
    memory: "128Mi"
    cpu: "500m"
securityContext:
  capabilities:
    add:
    - SYS_TRACE
    drop:
    - SYS_NICE
workdir: /app`)
	d, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if d.Name != "deployment" {
		t.Errorf("name was not parsed: %+v", d)
	}

	if len(d.Command) != 1 || d.Command[0] != "uwsgi" {
		t.Errorf("command was not parsed: %+v", d)
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

	if d.Annotations["key1"] != "value1" && d.Annotations["key2"] != "value2" {
		t.Errorf("Annotations were not parsed correctly")
	}

	if !reflect.DeepEqual(d.SecurityContext.Capabilities.Add, []apiv1.Capability{"SYS_TRACE"}) {
		t.Errorf("SecurityContext.Capabilities.Add was not parsed correctly. Expected [SYS_TRACE]")
	}

	if !reflect.DeepEqual(d.SecurityContext.Capabilities.Drop, []apiv1.Capability{"SYS_NICE"}) {
		t.Errorf("SecurityContext.Capabilities.Drop was not parsed correctly. Expected [SYS_NICE]")
	}
}

func Test_extraArgs(t *testing.T) {
	manifest := []byte(`
name: deployment
container: core
image: code/core:0.1.8
command: ["uwsgi"]
requests:
    memory: "64Mi"
    cpu: "250m"
  limits:
    memory: "128Mi"
    cpu: "500m"
workdir: /app`)
	_, err := Read(manifest)
	if err == nil {
		t.Errorf("manifest with bad attribute didn't fail to load")
	}
}

func Test_loadDevDefaults(t *testing.T) {
	var tests = []struct {
		name                string
		manifest            []byte
		expectedEnvironment []EnvVar
		expectedForward     []Forward
	}{
		{
			"long script",
			[]byte(`name: service
container: core
workdir: /app`),
			[]EnvVar{},
			[]Forward{},
		},
		{
			"basic script",
			[]byte(`name: service
container: core
workdir: /app`),
			[]EnvVar{},
			[]Forward{},
		},
		{
			"env vars",
			[]byte(`name: service
container: core
workdir: /app
environment:
  - ENV=production
  - name=test-node`),
			[]EnvVar{
				{Name: "ENV", Value: "production"},
				{Name: "name", Value: "test-node"},
			},
			[]Forward{},
		},
		{
			"forward",
			[]byte(`name: service
container: core
workdir: /app
forward:
  - 9000:8000
  - 9001:8001`),
			[]EnvVar{},
			[]Forward{
				{Local: 9000, Remote: 8000},
				{Local: 9001, Remote: 8001},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			if len(d.Command) != 1 || d.Command[0] != "sh" {
				t.Errorf("command was parsed: %+v", d)
			}

			if !reflect.DeepEqual(d.Environment, tt.expectedEnvironment) {
				t.Errorf("environment was not parsed correctly:\n%+v\n%+v", d.Environment, tt.expectedEnvironment)
			}

			if !reflect.DeepEqual(d.Forward, tt.expectedForward) {
				t.Errorf("environment was not parsed correctly:\n%+v\n%+v", d.Forward, tt.expectedForward)
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

func Test_loadDevImage(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		image    string
		tagValue string
	}{
		{
			name:     "tag",
			want:     "code/core:dev",
			image:    "code/core:${tag}",
			tagValue: "dev",
		},
		{
			name:     "registry",
			want:     "dev/core:latest",
			image:    "${tag}/core:latest",
			tagValue: "dev",
		},
		{
			name:     "full",
			want:     "dev/core:latest",
			image:    "${tag}",
			tagValue: "dev/core:latest",
		},
		{
			name:     "missing",
			want:     "code/core:",
			image:    "code/core:${image}",
			tagValue: "tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := []byte(fmt.Sprintf(`
name: deployment
image: %s
`, tt.image))
			os.Setenv("tag", tt.tagValue)
			d, err := Read(manifest)
			if err != nil {
				t.Fatal(err)
			}

			if d.Image != tt.want {
				t.Errorf("got: %s, expected: %s", d.Image, tt.want)
			}
		})
	}
}

func TestDev_validate(t *testing.T) {
	tests := []struct {
		name    string
		devName string
		wantErr bool
	}{
		{name: "empty", devName: "", wantErr: true},
		{name: "starts-with-dash", devName: "-bad-name", wantErr: true},
		{name: "ends-with-dash", devName: "bad-name-", wantErr: true},
		{name: "symbols", devName: "1$good-2", wantErr: true},
		{name: "alphanumeric", devName: "good-2", wantErr: false},
		{name: "good", devName: "good-name", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &Dev{
				Name:            tt.devName,
				ImagePullPolicy: apiv1.PullAlways,
			}
			if err := dev.validate(); (err != nil) != tt.wantErr {
				t.Errorf("Dev.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_LoadRemote(t *testing.T) {
	manifest := []byte(`
  name: deployment
  container: core
  image: code/core:0.1.8
  command: ["uwsgi"]
  remote: 22100
  annotations:
    key1: value1
    key2: value2
  forward:
    - 8080:8080
  resources:
    requests:
      memory: "64Mi"
      cpu: "250m"
    limits:
      memory: "128Mi"
      cpu: "500m"
  environment:
    - env=development
  securityContext:
    capabilities:
      add:
      - SYS_TRACE
      drop:
      - SYS_NICE
  workdir: /app`)
	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	dev.LoadRemote()

	if dev.Command[0] != "/var/okteto/bin/remote" {
		t.Errorf("remote command wasn't set: %s", dev.Command)
	}

	if len(dev.Forward) != 2 {
		t.Errorf("forward wasn't injected")
	}

	if dev.Forward[1].Local != 22100 {
		t.Errorf("local forward wasn't 22100 it was %d", dev.Forward[1].Local)
	}
}

func Test_LoadForcePull(t *testing.T) {
	manifest := []byte(`
  name: a
  annotations:
    key1: value1
  services:
    - name: b
      imagePullPolicy: IfNotPresent`)
	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	dev.LoadForcePull()

	if dev.ImagePullPolicy != apiv1.PullAlways {
		t.Errorf("wrong image pull policy for main container: %s", dev.ImagePullPolicy)
	}

	if dev.Annotations[OktetoRestartAnnotation] == "" {
		t.Errorf("restart annotation not set for main container")
	}

	dev = dev.Services[0]
	if dev.ImagePullPolicy != apiv1.PullAlways {
		t.Errorf("wrong image pull policy for services: %s", dev.ImagePullPolicy)
	}

	if dev.Annotations[OktetoRestartAnnotation] == "" {
		t.Errorf("restart annotation not set for services")
	}
}

func Test_remoteEnabled(t *testing.T) {
	dev := &Dev{}

	if dev.RemoteModeEnabled() {
		t.Errorf("default should be remote disabled")
	}

	dev = &Dev{RemotePort: 22000}

	if !dev.RemoteModeEnabled() {
		t.Errorf("remote should be enabled after adding a port")
	}
}
