// Copyright 2023 The Okteto Authors
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
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/compose-spec/godotenv"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

func Test_LoadManifest(t *testing.T) {
	manifestBytes := []byte(`
name: deployment
container: core
image: code/core:0.1.8
command: ["uwsgi"]
annotations:
  key1: value1
  key2: value2
labels:
  key3: value3
metadata:
  labels:
    key4: value4
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
serviceAccount: sa
workdir: /app
persistentVolume:
  enabled: true
timeout: 63s
services:
  - name: deployment
    container: core
    image: code/core:0.1.8
    command: ["uwsgi"]
    annotations:
      key1: value1
      key2: value2
    labels:
      key3: value3
    metadata:
      labels:
        key4: value4
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
`)
	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	main := manifest.Dev["deployment"]

	if len(main.Services) != 1 {
		t.Errorf("'services' was not parsed: %+v", main)
	}
	for _, dev := range []*Dev{main, main.Services[0]} {
		if dev.Name != "deployment" {
			t.Errorf("'name' was not parsed: %+v", main)
		}

		if len(dev.Command.Values) != 1 || dev.Command.Values[0] != "uwsgi" {
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

		if dev.Metadata.Annotations["key1"] != "value1" && dev.Metadata.Annotations["key2"] != "value2" {
			t.Errorf("Annotations were not parsed correctly")
		}
		if dev.Metadata.Labels["key4"] != "value4" {
			t.Errorf("Labels were not parsed correctly")
		}

		if dev.Selector["key3"] != "value3" {
			t.Errorf("Selector were not parsed correctly")
		}
	}

	expected := (63 * time.Second)
	if expected != main.Timeout.Default {
		t.Errorf("the default timeout wasn't applied, got %s, expected %s", main.Timeout, expected)
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

func Test_LoadManifestDefaults(t *testing.T) {
	tests := []struct {
		name                string
		manifest            []byte
		expectedEnvironment env.Environment
		expectedForward     []forward.Forward
	}{
		{
			"long script",
			[]byte(`name: service
container: core
workdir: /app`),
			env.Environment{},
			[]forward.Forward{},
		},
		{
			"basic script",
			[]byte(`name: service
container: core
workdir: /app`),
			env.Environment{},
			[]forward.Forward{},
		},
		{
			"env vars",
			[]byte(`name: service
container: core
workdir: /app
environment:
  - ENV=production
  - name=test-node`),
			env.Environment{
				{Name: "ENV", Value: "production"},
				{Name: "name", Value: "test-node"},
			},
			[]forward.Forward{},
		},
		{
			"forward",
			[]byte(`name: service
container: core
workdir: /app
forward:
  - 9000:8000
  - 9001:8001`),
			env.Environment{},
			[]forward.Forward{
				{Local: 9000, Remote: 8000, Service: false, ServiceName: ""},
				{Local: 9001, Remote: 8001, Service: false, ServiceName: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := Read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			d := manifest.Dev["service"]

			if len(d.Command.Values) != 1 || d.Command.Values[0] != "sh" {
				t.Errorf("command was parsed: %+v", d)
			}

			for _, env := range d.Environment {
				found := false
				for _, expectedEnv := range tt.expectedEnvironment {
					if env.Name == expectedEnv.Name && env.Value == expectedEnv.Value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("environment was not parsed correctly:\n%+v\n%+v", d.Environment, tt.expectedEnvironment)
				}
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

			if !d.PersistentVolumeEnabled() {
				t.Errorf("persistent volume was not enabled by default")
			}

			defaultTimeout, err := GetTimeout()
			assert.NoError(t, err)
			if defaultTimeout != d.Timeout.Default {
				t.Errorf("the default timeout wasn't applied, got %s, expected %s", d.Timeout, defaultTimeout)
			}
		})
	}
}

func Test_loadName(t *testing.T) {
	tests := []struct {
		name      string
		devName   string
		value     string
		want      string
		onService bool
	}{
		{
			name:    "no-var",
			devName: "code",
			value:   "1",
			want:    "code",
		},
		{
			name:    "var",
			devName: "code-${value}",
			value:   "1",
			want:    "code-1",
		},
		{
			name:    "missing",
			devName: "code-${valueX}",
			value:   "1",
			want:    "code-",
		},
		{
			name:      "no-var-vc",
			devName:   "code",
			value:     "1",
			onService: true,
			want:      "code",
		},
		{
			name:      "var-svc",
			devName:   "code-${value}",
			value:     "1",
			onService: true,
			want:      "code-1",
		},
		{
			name:      "missing-svc",
			devName:   "code-${valueX}",
			value:     "1",
			onService: true,
			want:      "code-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestBytes := []byte(fmt.Sprintf(`
name: %s`, tt.devName))

			devName := tt.want

			if tt.onService {
				manifestBytes = []byte(fmt.Sprintf(`
name: n1
services:
  - name: %s`, tt.devName))
				devName = "n1"
			}

			t.Setenv("value", tt.value)
			manifest, err := Read(manifestBytes)
			if err != nil {
				t.Fatal(err)
			}

			dev := manifest.Dev[devName]

			name := dev.Name
			if tt.onService {
				name = dev.Services[0].Name
			}

			if name != tt.want {
				t.Errorf("got: '%s', expected: '%s'", name, tt.want)
			}
		})
	}
}

func Test_loadSelector(t *testing.T) {
	tests := []struct {
		selector Selector
		want     Selector
		name     string
		value    string
	}{
		{
			name:     "no-var",
			selector: Selector{"a": "1", "b": "2"},
			value:    "3",
			want:     Selector{"a": "1", "b": "2"},
		},
		{
			name:     "var",
			selector: Selector{"a": "1", "b": "${value}"},
			value:    "3",
			want:     Selector{"a": "1", "b": "3"},
		},
		{
			name:     "missing",
			selector: Selector{"a": "1", "b": "${valueX}"},
			value:    "1",
			want:     Selector{"a": "1", "b": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &Dev{Selector: tt.selector}
			t.Setenv("value", tt.value)
			if err := dev.loadSelector(); err != nil {
				t.Fatalf("couldn't load selector")
			}

			for key, value := range dev.Labels {
				if tt.want[key] != value {
					t.Errorf("got: '%v', expected: '%v'", dev.Labels, tt.want)
				}
			}
		})
	}
}

func Test_loadImage(t *testing.T) {
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
			manifestBytes := []byte(fmt.Sprintf(`
name: deployment
image: %s
`, tt.image))

			if tt.onService {
				manifestBytes = []byte(fmt.Sprintf(`
name: deployment
image: image
services:
  - name: svc
    image: %s
`, tt.image))
			}

			t.Setenv("tag", tt.tagValue)
			manifest, err := Read(manifestBytes)
			if err != nil {
				t.Fatal(err)
			}

			dev := manifest.Dev["deployment"]

			img := dev.Image
			if tt.onService {
				img = dev.Services[0].Image
			}

			if img.Name != tt.want {
				t.Errorf("got: '%s', expected: '%s'", img.Name, tt.want)
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
				Image:           &build.Info{},
				Push:            &build.Info{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  ".",
							RemotePath: "/app",
						},
					},
				},
			}
			// Since dev isn't being unmarshalled through Read, apply defaults
			// before validating.
			if err := dev.SetDefaults(); err != nil {
				t.Fatalf("error applying defaults: %v", err)
			}
			if err := dev.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Dev.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDev_validateReplicas(t *testing.T) {
	replicasNumber := 5
	dev := &Dev{
		Name:            "test",
		ImagePullPolicy: apiv1.PullAlways,
		Image:           &build.Info{},
		Push:            &build.Info{},
		Replicas:        &replicasNumber,
		Sync: Sync{
			Folders: []SyncFolder{
				{
					LocalPath:  ".",
					RemotePath: "/app",
				},
			},
		},
	}
	// Since dev isn't being unmarshalled through Read, apply defaults
	// before validating.
	if err := dev.SetDefaults(); err != nil {
		t.Fatalf("error applying defaults: %v", err)
	}
	if err := dev.Validate(); err == nil {
		t.Errorf("Dev.validate() error = %v, wantErr %v", err, true)
	}

}

func TestDev_readImageContext(t *testing.T) {
	tests := []struct {
		expected *build.Info
		name     string
		manifest []byte
	}{
		{
			name: "context pointing to url",
			manifest: []byte(`name: deployment
image:
  context: https://github.com/okteto/okteto.git
`),
			expected: &build.Info{
				Context: "https://github.com/okteto/okteto.git",
			},
		},
		{
			name: "context pointing to path",
			manifest: []byte(`name: deployment
image:
  context: .
`),
			expected: &build.Info{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := Read(tt.manifest)
			if err != nil {
				t.Fatalf("Wrong unmarshalling: %s", err.Error())
			}

			dev := manifest.Dev["deployment"]

			// Since dev isn't being unmarshalled through Read, apply defaults
			// before validating.
			if err := dev.SetDefaults(); err != nil {
				t.Fatalf("error applying defaults: %v", err)
			}
			if !reflect.DeepEqual(dev.Image, tt.expected) {
				t.Fatalf("Expected %v but got %v", tt.expected, dev.Image)
			}
		})
	}
}

func Test_LoadRemote(t *testing.T) {
	manifestBytes := []byte(`
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
	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	dev := manifest.Dev["deployment"]

	dev.LoadRemote("/tmp/key.pub")

	if dev.Command.Values[0] != "uwsgi" {
		t.Errorf("command wasn't set: %s", dev.Command.Values)
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
	manifestBytes := []byte(`
  name: deployment
  container: core
  image: code/core:0.1.8
  command: ["uwsgi"]
  annotations:
    key1: value1
    key2: value2
  reverse:
    - 8080:8080`)
	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	dev := manifest.Dev["deployment"]

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
	manifestBytes := []byte(`
  name: a
  annotations:
    key1: value1
  services:
    - name: b
      imagePullPolicy: IfNotPresent`)
	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	dev := manifest.Dev["a"]

	dev.LoadForcePull()

	if dev.ImagePullPolicy != apiv1.PullAlways {
		t.Errorf("wrong image pull policy for main container: %s", dev.ImagePullPolicy)
	}

	if dev.Metadata.Annotations[OktetoRestartAnnotation] == "" {
		t.Errorf("restart annotation not set for main container")
	}

	dev = dev.Services[0]
	if dev.ImagePullPolicy != apiv1.PullAlways {
		t.Errorf("wrong image pull policy for services: %s", dev.ImagePullPolicy)
	}

	if dev.Metadata.Annotations[OktetoRestartAnnotation] == "" {
		t.Errorf("restart annotation not set for services")
	}
}

func Test_validate(t *testing.T) {
	file, err := os.CreateTemp("", "okteto-secret-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	dir := t.TempDir()

	tests := []struct {
		name      string
		manifest  []byte
		expectErr bool
	}{
		{
			name: "services-with-disabled-pvc",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      persistentVolume:
        enabled: false
      services:
        - name: foo
          sync:
            - .:/app`),
			expectErr: true,
		},
		{
			name: "services-with-enabled-pvc",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      services:
        - name: foo
          sync:
            - .:/app`),
			expectErr: false,
		},
		{
			name: "pvc-size",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      persistentVolume:
        enabled: true
        size: 10Gi`),
			expectErr: false,
		},
		{
			name: "volumes-mount-path-/",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      volumes:
        - /`),
			expectErr: true,
		},
		{
			name: "volumes-relative-mount-path",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      volumes:
        - path`),
			expectErr: true,
		},
		{
			name: "external-volumes-mount-path-/",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      externalVolumes:
        - name:/`),
			expectErr: true,
		},
		{
			name: "external-volumes-relative-mount-path",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      externalVolumes:
        - name:path`),
			expectErr: true,
		},
		{
			name: "wrong-pvc-size",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      persistentVolume:
        enabled: true
        size: wrong`),
			expectErr: true,
		},
		{
			name: "services-with-mountpath-pullpolicy",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      services:
        - name: foo
          sync:
            - .:/app
          imagePullPolicy: Always`),
			expectErr: false,
		},
		{
			name: "services-with-bad-pullpolicy",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      services:
        - name: foo
          sync:
            - .:/app
          imagePullPolicy: Sometimes`),
			expectErr: true,
		},
		{
			name: "volumes",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
        - docs:/docs`),
			expectErr: true,
		},
		{
			name: "external-volumes",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      externalVolumes:
        - pvc1:path:/path
        - pvc2:/path`),
			expectErr: false,
		},
		{
			name: "secrets",
			manifest: []byte(fmt.Sprintf(`
      name: deployment
      sync:
        - .:/app
      secrets:
        - %s:/remote
        - %s:/remote`, file.Name(), file.Name())),
			expectErr: true,
		},
		{
			name: "bad-pull-policy",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      imagePullPolicy: what`),
			expectErr: true,
		},
		{
			name: "good-pull-policy",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      imagePullPolicy: IfNotPresent`),
			expectErr: false,
		},
		{
			name: "valid-ssh-server-port",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      sshServerPort: 2222`),
			expectErr: false,
		},
		{
			name: "invalid-ssh-server-port",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      sshServerPort: -1`),
			expectErr: true,
		},
		{
			name: "runAsNonRoot-with-root-user",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      securityContext:
        runAsNonRoot: true
        runAsUser: 0`),
			expectErr: true,
		},
		{
			name: "runAsNonRoot-with-non-root-user",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      securityContext:
        runAsNonRoot: true
        runAsUser: 101`),
			expectErr: false,
		},
		{
			name: "file",
			manifest: []byte(fmt.Sprintf(`
      name: deployment
      sync:
        - %s:/app`, file.Name())),
			expectErr: true,
		},
		{
			name: "dir",
			manifest: []byte(fmt.Sprintf(`
      name: deployment
      sync:
        - %s:/app`, dir)),
			expectErr: false,
		},
		{
			name: "runAsNonRoot-with-root-group",
			manifest: []byte(`
      name: deployment
      sync:
        - .:/app
      securityContext:
        runAsNonRoot: true
        runAsGroup: 0`),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := Read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			dev := manifest.Dev["deployment"]

			err = dev.Validate()
			if tt.expectErr && err == nil {
				t.Error("didn't get the expected error")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("got an unexpected error: %s", err)
			}
		})
	}
}

func TestPersistentVolumeEnabled(t *testing.T) {
	tests := []struct {
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
			expected: true,
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
			manifest, err := Read(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}
			dev := manifest.Dev["deployment"]

			if dev.PersistentVolumeEnabled() != tt.expected {
				t.Errorf("Expecting %t but got %t", tt.expected, dev.PersistentVolumeEnabled())
			}
		})
	}
}

func TestGetTimeout(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    time.Duration
		wantErr bool
	}{
		{name: "default value", want: 60 * time.Second},
		{name: "env var", want: 134 * time.Second, env: "134s"},
		{name: "bad env var", wantErr: true, env: "bad value"},
	}

	original := os.Getenv(OktetoTimeoutEnvVar)
	defer t.Setenv(OktetoTimeoutEnvVar, original)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv(OktetoTimeoutEnvVar, tt.env)
			}
			got, err := GetTimeout()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("GetTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loadEnvFile(t *testing.T) {
	tests := []struct {
		content   map[string]string
		existing  map[string]string
		expected  map[string]string
		name      string
		expectErr bool
	}{
		{
			name:      "missing",
			expectErr: true,
		},
		{
			name:      "basic",
			expectErr: false,
			content:   map[string]string{"foo": "bar"},
			expected:  map[string]string{"foo": "bar"},
		},
		{
			name:      "doesnt-override",
			expectErr: false,
			content:   map[string]string{"foo": "bar"},
			existing:  map[string]string{"foo": "var"},
			expected:  map[string]string{"foo": "var"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.content != nil {
				file, err := createEnvFile(tt.content)
				if err != nil {
					t.Fatal(err)
				}

				defer os.Remove(file)
			}

			for k, v := range tt.existing {
				t.Setenv(k, v)
			}

			if err := godotenv.Load(); err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}

			if tt.expectErr {
				t.Fatal("call didn't fail as expected")
			}

			for k, v := range tt.expected {
				got := os.Getenv(k)
				if got != v {
					t.Errorf("got %s=%s, expected %s=%s", k, got, k, v)
				}
			}
		})
	}
}

func Test_LoadManifestWithEnvFile(t *testing.T) {
	content := map[string]string{
		"DEPLOYMENT":    "main",
		"IMAGE_TAG":     "1.2",
		"MY_VAR":        "from-env-file",
		"SERVICE":       "secondary",
		"SERVICE_IMAGE": "code/service:2.1",
	}

	f, err := createEnvFile(content)
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f)

	manifestBytes := []byte(`
name: deployment-$DEPLOYMENT
container: core
image: code/core:$IMAGE_TAG
command: ["uwsgi"]
environment:
- MY_VAR=$MY_VAR
services:
  - name: deployment-$SERVICE
    container: core
    image: $SERVICE_IMAGE
    command: ["uwsgi"]
    environment:
    - MY_VAR=$MY_VAR`)

	if err := godotenv.Load(); err != nil {
		t.Fatal(err)
	}

	manifest, err := Read(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	main := manifest.Dev["deployment-main"]

	if len(main.Services) != 1 {
		t.Errorf("'services' was not parsed: %+v", main)
	}

	if main.Name != "deployment-main" {
		t.Errorf("'name' was not parsed: got %s, expected %s", main.Name, "deployment-main")
	}

	if main.Image.Name != "code/core:1.2" {
		t.Errorf("'tag' was not parsed: got %s, expected %s", main.Image.Name, "code/core:1.2")
	}

	if main.Environment[0].Value != "from-env-file" {
		t.Errorf("'environment' was not parsed: got %s, expected %s", main.Environment[0].Value, "from-env-file")
	}

	if main.Services[0].Name != "deployment-secondary" {
		t.Errorf("'name' was not parsed: got %s, expected %s", main.Services[0].Name, "deployment-main")
	}

	if main.Services[0].Image.Name != "code/service:2.1" {
		t.Errorf("'tag' was not parsed: got %s, expected %s", main.Services[0].Image.Name, "code/service:2.1")
	}

	if main.Services[0].Environment[0].Value != "from-env-file" {
		t.Errorf("'environment' was not parsed: got %s, expected %s", main.Services[0].Environment[0].Value, "from-env-file")
	}
}

func Test_validateForExtraFields(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name: "services",
			value: `services:
    - name: 1`,
		},
		{
			name:  "autocreate",
			value: "autocreate: true",
		},
		{
			name:  "context",
			value: "context: minikube",
		},
		{
			name: "push",
			value: `push:
                   context: .
                   dockerfile: Dockerfile
                   target: prod`,
		},
		{
			name:  "healthchecks",
			value: "healthchecks: true",
		},
		{
			name: "probes",
			value: `probes:
               liveness: true
               readiness: true
               startup: true`,
		},
		{
			name: "lifecycle",
			value: `lifecycle:
               postStart: false
               postStop: true`,
		},
		{
			name: "securityContext",
			value: `securityContext:
               runAsUser: 1000
               runAsGroup: 2000
               fsGroup: 3000
               allowPrivilegeEscalation: false
               capabilities:
                 add:
                 - SYS_PTRACE`,
		},
		{
			name:  "serviceAccount",
			value: "serviceAccount: sa",
		},
		{
			name:  "remote",
			value: "remote: 2222",
		},
		{
			name:  "sshServerPort",
			value: "sshServerPort: 2222",
		},
		{
			name:  "externalVolumes",
			value: `externalVolumes: []`,
		},
		{
			name: "forward",
			value: `forward:
                   - 8080:80`,
		},
		{
			name: "reverse",
			value: `reverse:
                   - 9000:9001`,
		},
		{
			name: "reverse",
			value: `reverse:
                   - 9000:9001`,
		},
		{
			name:  "interface",
			value: "interface: 0.0.0.0",
		},
		{
			name: "persistentVolume",
			value: `persistentVolume:
                   enabled: true
                   storageClass: standard
                   size: 30Gi`,
		},
		{
			name: "initContainer",
			value: `initContainer:
                   image: alpine`,
		},
		{
			name:  "initFromImage",
			value: "initFromImage: true",
		},
		{
			name: "timeout",
			value: `timeout:
                   default: 3m
                   resources: 5m`,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s is present", tt.name), func(t *testing.T) {
			manifest := []byte(fmt.Sprintf(`
name: deployment
container: core
image: code/core:0.1.8
command: ["uwsgi"]
annotations:
  key1: value1
  key2: value2
labels:
  key3: value3
metadata:
  labels:
    key4: value4
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
serviceAccount: sa
workdir: /app
persistentVolume:
  enabled: true
timeout: 63s
services:
  - name: deployment
    container: core
    image: code/core:0.1.8
    command: ["uwsgi"]
    annotations:
      key1: value1
      key2: value2
    labels:
      key3: value3
    metadata:
      labels:
        key4: value4
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
    workdir: /app
    %s`, tt.value))
			expected := fmt.Sprintf("error on dev 'deployment': %q is not supported in Services. Please visit https://www.okteto.com/docs/reference/okteto-manifest/#services-object-optional for documentation", tt.name)

			_, err := Read(manifest)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, expected)
		})
	}
}

func createEnvFile(content map[string]string) (string, error) {
	file, err := os.OpenFile(".env", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}

	for k, v := range content {
		_, err = file.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		if err != nil {
			return "", err
		}
	}

	if err := file.Sync(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func Test_expandEnvFiles(t *testing.T) {
	tests := []struct {
		name     string
		dev      *Dev
		envs     []byte
		expected env.Environment
	}{
		{
			name: "add new envs",
			dev: &Dev{
				Environment: env.Environment{},
				EnvFiles:    env.Files{},
			},
			envs: []byte("key1=value1"),
			expected: env.Environment{
				env.Var{
					Name:  "key1",
					Value: "value1",
				},
			},
		},
		{
			name: "dont overwrite envs",
			dev: &Dev{
				Environment: env.Environment{
					{
						Name:  "key1",
						Value: "value1",
					},
				},
				EnvFiles: env.Files{},
			},
			envs: []byte("key1=value100"),
			expected: env.Environment{
				env.Var{
					Name:  "key1",
					Value: "value1",
				},
			},
		},
		{
			name: "empty env - infer value",
			dev: &Dev{
				Environment: env.Environment{},
				EnvFiles:    env.Files{},
			},
			envs: []byte("OKTETO_TEST="),
			expected: env.Environment{
				env.Var{
					Name:  "OKTETO_TEST",
					Value: "myvalue",
				},
			},
		},
		{
			name: "empty env - empty value",
			dev: &Dev{
				Environment: env.Environment{},
				EnvFiles:    env.Files{},
			},
			envs:     []byte("OKTETO_TEST2="),
			expected: env.Environment{},
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s is present", tt.name), func(t *testing.T) {

			file, err := os.CreateTemp("", ".env")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(file.Name())

			tt.dev.EnvFiles = env.Files{file.Name()}

			t.Setenv("OKTETO_TEST", "myvalue")

			if _, err = file.Write(tt.envs); err != nil {
				t.Fatal("Failed to write to temporary file", err)
			}
			if err := tt.dev.expandEnvFiles(); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.expected, tt.dev.Environment)
		})
	}
}

func TestPrepare(t *testing.T) {
	type input struct {
		manifestPath string
	}
	tests := []struct {
		name          string
		dev           *Dev
		input         input
		expectedError bool
	}{
		{
			name: "success",
			dev:  &Dev{},
			input: input{
				manifestPath: "okteto.yml",
			},
			expectedError: false,
		},
		{
			name: "with missing envFiles",
			dev: &Dev{
				EnvFiles: env.Files{".notfound"},
			},
			input: input{
				manifestPath: "okteto.yml",
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dev.PreparePathsAndExpandEnvFiles(tt.input.manifestPath, afero.NewMemMapFs())
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
