// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"testing"

	apiv1 "k8s.io/api/core/v1"
)

func Test_LoadDev(t *testing.T) {
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
workdir: /app
persistentVolume:
  enabled: true
services:
  - name: deployment
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
	main, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(main.Services) != 1 {
		t.Errorf("'services' was not parsed: %+v", main)
	}
	for _, dev := range []*Dev{main, main.Services[0]} {
		if dev.Name != "deployment" {
			t.Errorf("'name' was not parsed: %+v", main)
		}

		if len(dev.Command) != 1 || dev.Command[0] != "uwsgi" {
			t.Errorf("command was not parsed: %+v", dev)
		}

		memory := dev.Resources.Requests["memory"]
		if memory.String() != "64Mi" {
			t.Errorf("Resources.Requests.Memory was not parsed: %s", memory.String())
		}

		cpu := dev.Resources.Requests["cpu"]
		if cpu.String() != "250m" {
			t.Errorf("Resources.Requests.CPU was not parsed correctly. Expected '250M', got '%s'", cpu.String())
		}

		memory = dev.Resources.Limits["memory"]
		if memory.String() != "128Mi" {
			t.Errorf("Resources.Requests.Memory was not parsed: %s", memory.String())
		}

		cpu = dev.Resources.Limits["cpu"]
		if cpu.String() != "500m" {
			t.Errorf("Resources.Requests.CPU was not parsed correctly. Expected '500M', got '%s'", cpu.String())
		}

		if dev.Annotations["key1"] != "value1" && dev.Annotations["key2"] != "value2" {
			t.Errorf("Annotations were not parsed correctly")
		}

		if !reflect.DeepEqual(dev.SecurityContext.Capabilities.Add, []apiv1.Capability{"SYS_TRACE"}) {
			t.Errorf("SecurityContext.Capabilities.Add was not parsed correctly. Expected [SYS_TRACE]")
		}

		if !reflect.DeepEqual(dev.SecurityContext.Capabilities.Drop, []apiv1.Capability{"SYS_NICE"}) {
			t.Errorf("SecurityContext.Capabilities.Drop was not parsed correctly. Expected [SYS_NICE]")
		}
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

func Test_LoadDevDefaults(t *testing.T) {
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
				{Local: 9000, Remote: 8000, Service: false, ServiceName: ""},
				{Local: 9001, Remote: 8001, Service: false, ServiceName: ""},
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

			if d.PersistentVolumeEnabled() {
				t.Errorf("peristent volume was enabled by default")
			}
		})
	}
}

func Test_loadDevImage(t *testing.T) {
	tests := []struct {
		name      string
		want      string
		image     string
		tagValue  string
		onService bool
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
			name:     "missing-tag",
			want:     "code/core:",
			image:    "code/core:${image}",
			tagValue: "tag",
		},
		{
			name:      "tag-svc",
			want:      "code/core:dev",
			image:     "code/core:${tag}",
			tagValue:  "dev",
			onService: true,
		},
		{
			name:      "registry-svc",
			want:      "dev/core:latest",
			image:     "${tag}/core:latest",
			tagValue:  "dev",
			onService: true,
		},
		{
			name:      "full-svc",
			want:      "dev/core:latest",
			image:     "${tag}",
			tagValue:  "dev/core:latest",
			onService: true,
		},
		{
			name:      "missing-tag-svc",
			want:      "code/core:",
			image:     "code/core:${image}",
			tagValue:  "tag",
			onService: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := []byte(fmt.Sprintf(`
name: deployment
image: %s
`, tt.image))

			if tt.onService {
				manifest = []byte(fmt.Sprintf(`
name: deployment
image: image
services:
  - name: svc
    image: %s
`, tt.image))
			}

			os.Setenv("tag", tt.tagValue)
			d, err := Read(manifest)
			if err != nil {
				t.Fatal(err)
			}

			img := d.Image
			if tt.onService {
				img = d.Services[0].Image
			}

			if img != tt.want {
				t.Errorf("got: '%s', expected: '%s'", img, tt.want)
			}
		})
	}
}

func TestDev_validateName(t *testing.T) {
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
				Push:            &BuildInfo{},
			}
			// Since dev isn't being unmarshalled through Read, apply defaults
			// before validating.
			if err := dev.setDefaults(); err != nil {
				t.Fatalf("error applying defaults: %v", err)
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
  sshServerPort: 2222
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

	dev.LoadRemote("/tmp/key.pub")

	if dev.Command[0] != "uwsgi" {
		t.Errorf("command wasn't set: %s", dev.Command)
	}

	if len(dev.Forward) != 1 {
		t.Errorf("forward was injected")
	}

	if dev.RemotePort != 22100 {
		t.Errorf("local remote port wasn't 22100 it was %d", dev.RemotePort)
	}

	if dev.SSHServerPort != 2222 {
		t.Errorf("server remote port wasn't 2222 it was %d", dev.SSHServerPort)
	}

	if dev.Secrets[0].LocalPath != "/tmp/key.pub" {
		t.Errorf("local key was not set correctly: %s", dev.Secrets[0].LocalPath)
	}

	if dev.Secrets[0].RemotePath != "/var/okteto/remote/authorized_keys" {
		t.Errorf("remote key was not set at /var/okteto/remote/authorized_keys")
	}
}

func Test_Reverse(t *testing.T) {
	manifest := []byte(`
  name: deployment
  container: core
  image: code/core:0.1.8
  command: ["uwsgi"]
  annotations:
    key1: value1
    key2: value2
  reverse:
    - 8080:8080`)
	dev, err := Read(manifest)
	if err != nil {
		t.Fatal(err)
	}

	dev.LoadRemote("/tmp/key.pub")

	if dev.RemotePort == 0 {
		t.Error("remote port was not automatically enabled")
	}

	if dev.Reverse[0].Local != 8080 {
		t.Errorf("remote forward local wasn't 8080 it was %d", dev.Reverse[0].Local)
	}

	if dev.Reverse[0].Remote != 8080 {
		t.Errorf("remote forward remote wasn't 8080 it was %d", dev.Reverse[0].Remote)
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

func TestRemoteEnabled(t *testing.T) {
	var dev *Dev
	if dev.RemoteModeEnabled() {
		t.Errorf("nil should be remote disabled")
	}

	dev = &Dev{}

	if dev.RemoteModeEnabled() {
		t.Errorf("default should be remote disabled")
	}

	dev = &Dev{RemotePort: 22000}

	if !dev.RemoteModeEnabled() {
		t.Errorf("remote should be enabled after adding a port")
	}

	dev = &Dev{Reverse: []Reverse{{Local: 22000, Remote: 22000}}}

	if !dev.RemoteModeEnabled() {
		t.Errorf("remote should be enabled after adding a remote forward")
	}
}

func Test_validate(t *testing.T) {
	file, err := ioutil.TempFile("/tmp", "okteto-secret-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	var tests = []struct {
		name      string
		manifest  []byte
		expectErr bool
	}{
		{
			name: "services-with-disabled-pvc",
			manifest: []byte(`
      name: deployment
      persistentVolume:
        enabled: false
      services:
        - name: foo`),
			expectErr: true,
		},
		{
			name: "services-with-enabled-pvc",
			manifest: []byte(`
      name: deployment
      persistentVolume:
        enabled: true
      services:
        - name: foo`),
			expectErr: false,
		},
		{
			name: "pvc-size",
			manifest: []byte(`
      name: deployment
      persistentVolume:
        enabled: true
        size: 10Gi`),
			expectErr: false,
		},
		{
			name: "wrong-pvc-size",
			manifest: []byte(`
      name: deployment
      persistentVolume:
        enabled: true
        size: wrong`),
			expectErr: true,
		},
		{
			name: "services-with-mountpath-pullpolicy",
			manifest: []byte(`
      name: deployment
      persistentVolume:
        enabled: true
      services:
        - name: foo
          imagePullPolicy: Always`),
			expectErr: false,
		},
		{
			name: "services-with-bad-pullpolicy",
			manifest: []byte(`
      name: deployment
      services:
        - name: foo
          imagePullPolicy: Sometimes`),
			expectErr: true,
		},
		{
			name: "volumes",
			manifest: []byte(`
      name: deployment
      persistentVolume:
        enabled: true
      volumes:
        - docs:/docs`),
			expectErr: false,
		},
		{
			name: "secrets",
			manifest: []byte(fmt.Sprintf(`
      name: deployment
      secrets:
        - %s:/remote
        - %s:/remote`, file.Name(), file.Name())),
			expectErr: true,
		},
		{
			name: "bad-pull-policy",
			manifest: []byte(`
      name: deployment
      imagePullPolicy: what`),
			expectErr: true,
		},
		{
			name: "good-pull-policy",
			manifest: []byte(`
      name: deployment
      imagePullPolicy: IfNotPresent`),
			expectErr: false,
		},
		{
			name: "subpath-on-main-dev",
			manifest: []byte(`
      name: deployment
      subpath: /app/docs`),
			expectErr: true,
		},
		{
			name: "valid-ssh-server-port",
			manifest: []byte(`
      name: deployment
      sshServerPort: 2222`),
			expectErr: false,
		},
		{
			name: "invalid-ssh-server-port",
			manifest: []byte(`
      name: deployment
      sshServerPort: -1`),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := Read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			err = dev.validate()
			if tt.expectErr && err == nil {
				t.Error("didn't got the expected error")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("got an unexpected error: %s", err)
			}

		})
	}
}
func TestPersistentVolumeEnabled(t *testing.T) {
	var tests = []struct {
		name     string
		manifest []byte
		expected bool
	}{
		{
			name: "default",
			manifest: []byte(`
      name: deployment
      container: core
      image: code/core:0.1.8`),
			expected: false,
		},
		{
			name: "set",
			manifest: []byte(`
      name: deployment
      container: core
      image: code/core:0.1.8
      persistentVolume:
        enabled: true`),
			expected: true,
		},
		{
			name: "disabled",
			manifest: []byte(`
      name: deployment
      container: core
      image: code/core:0.1.8
      persistentVolume:
        enabled: false`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := Read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			if dev.PersistentVolumeEnabled() != tt.expected {
				t.Errorf("Expecting %t but got %t", tt.expected, dev.PersistentVolumeEnabled())
			}
		})
	}
}

func Test_fullSubPath(t *testing.T) {
	var tests = []struct {
		name    string
		i       int
		subPath string
		want    string
	}{
		{
			name:    "source-code-without-subpath",
			i:       0,
			subPath: "",
			want:    SourceCodeSubPath,
		},
		{
			name:    "source-code-with-subpath",
			i:       0,
			subPath: "data",
			want:    path.Join(SourceCodeSubPath, "data"),
		},
		{
			name:    "data-without-subpath",
			i:       2,
			subPath: "",
			want:    "volume-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fullSubPath(tt.i, tt.subPath)
			if result != tt.want {
				t.Errorf("error in test '%s', expected '%s' vs '%s'", tt.name, tt.want, result)
			}
		})
	}
}
