// Copyright 2021 The Okteto Authors
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

package manifest

import (
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model/build"
	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/dev"
	"github.com/okteto/okteto/pkg/model/environment"
	"github.com/okteto/okteto/pkg/model/metadata"
	"github.com/okteto/okteto/pkg/model/port"
	"github.com/okteto/okteto/pkg/model/secrets"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestManifestUnmarshalling(t *testing.T) {
	tests := []struct {
		name            string
		manifest        []byte
		expected        *Manifest
		isErrorExpected bool
	}{
		{
			name: "manifest with namespace and context",
			manifest: []byte(`
namespace: test
context: context-to-use
deploy:
  - okteto stack deploy`),
			expected: &Manifest{
				Namespace: "test",
				Deploy: &Deploy{
					Commands: []string{
						"okteto stack deploy",
					},
				},
				Devs:    map[string]*dev.Dev{},
				Build:   Build{},
				Context: "context-to-use",
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with dev and deploy",
			manifest: []byte(`
deploy:
  - okteto stack deploy
dev:
  test-1:
    sync:
    - app:/app
  test-2:
    sync:
    - app:/app
`),
			expected: &Manifest{
				Build: Build{},
				Deploy: &Deploy{
					Commands: []string{
						"okteto stack deploy",
					},
				},
				Devs: map[string]*dev.Dev{
					"test-1": {
						Name: "test-1",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]secrets.Secret, 0),
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*dev.Dev{},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
					"test-2": {
						Name: "test-2",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]secrets.Secret, 0),
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*dev.Dev{},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
				},
			},

			isErrorExpected: false,
		},
		{
			name: "only dev",
			manifest: []byte(`name: test
sync:
  - app:/app`),
			expected: &Manifest{
				Build: Build{},
				Devs: map[string]*dev.Dev{
					"test": {
						Name: "test",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]secrets.Secret, 0),
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*dev.Dev{},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "only dev with service",
			manifest: []byte(`name: test
sync:
  - app:/app
services:
  - name: svc`),
			expected: &Manifest{
				Build: Build{},
				Devs: map[string]*dev.Dev{
					"test": {
						Name: "test",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]secrets.Secret, 0),
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services: []*dev.Dev{
							{
								Name:            "svc",
								Annotations:     metadata.Annotations{},
								Selector:        dev.Selector{},
								EmptyImage:      true,
								Image:           &build.Build{},
								ImagePullPolicy: v1.PullAlways,
								Secrets:         []secrets.Secret{},
								Probes: &dev.Probes{
									Liveness:  false,
									Readiness: false,
									Startup:   false,
								},
								Lifecycle: &dev.Lifecycle{
									PostStart: false,
									PostStop:  false,
								},
								SecurityContext: &dev.SecurityContext{
									RunAsUser:    pointer.Int64(0),
									RunAsGroup:   pointer.Int64(0),
									RunAsNonRoot: nil,
									FSGroup:      pointer.Int64(0),
								},
								Sync: dev.Sync{
									RescanInterval: 300,
								},
								Forward:  []port.Forward{},
								Reverse:  []dev.Reverse{},
								Services: []*dev.Dev{},
								Metadata: &metadata.Metadata{
									Labels:      metadata.Labels{},
									Annotations: metadata.Annotations{},
								},
								Volumes: []dev.Volume{},
							},
						},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "only dev with service unsupported field",
			manifest: []byte(`
sync:
  - app:/app
services:
  - name: svc
    autocreate: true`),
			expected:        nil,
			isErrorExpected: true,
		},
		{
			name: "only dev with errors",
			manifest: []byte(`
sync:
  - app:/app
non-found-field:
  testing`),
			expected:        nil,
			isErrorExpected: true,
		},
		{
			name: "dev manifest with one dev",
			manifest: []byte(`
dev:
  test:
    sync:
    - app:/app
`),
			expected: &Manifest{
				Build: Build{},
				Devs: map[string]*dev.Dev{
					"test": {
						Name: "test",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Secrets: make([]secrets.Secret, 0),
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*dev.Dev{},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with multiple devs",
			manifest: []byte(`
dev:
  test-1:
    sync:
    - app:/app
  test-2:
    sync:
    - app:/app
`),
			expected: &Manifest{
				Build: Build{},
				Devs: map[string]*dev.Dev{
					"test-1": {
						Name: "test-1",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]secrets.Secret, 0),
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*dev.Dev{},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
					"test-2": {
						Name: "test-2",
						Sync: dev.Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []dev.SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []port.Forward{},
						Selector:        dev.Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: constants.Localhost,
						PersistentVolumeInfo: &dev.PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Build{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]secrets.Secret, 0),
						Command: dev.Command{Values: []string{"sh"}},
						Probes: &dev.Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &dev.Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &dev.SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*dev.Dev{},
						InitContainer: dev.InitContainer{
							Image: constants.OktetoBinImageTag,
						},
						Timeout: dev.Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &metadata.Metadata{
							Labels:      metadata.Labels{},
							Annotations: metadata.Annotations{},
						},
						Environment: environment.Environment{},
						Volumes:     []dev.Volume{},
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with errors",
			manifest: []byte(`
dev:
  test-1:
    sync:
    - app:/app
    services:
    - name: svc
  test-2:
    sync:
    - app:/app
    services:
    - name: svc
sync:
- app:test
`),
			expected:        nil,
			isErrorExpected: true,
		},
		{
			name: "dev manifest with deploy",
			manifest: []byte(`
deploy:
  - okteto stack deploy
`),
			expected: &Manifest{
				Devs: map[string]*dev.Dev{},
				Deploy: &Deploy{
					Commands: []string{
						"okteto stack deploy",
					},
				},
				Build: Build{},
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with deploy",
			manifest: []byte(`
deploy:
  - okteto stack deploy
devs:
  - api
  - test
`),
			expected: &Manifest{
				Devs: map[string]*dev.Dev{},
				Deploy: &Deploy{
					Commands: []string{
						"okteto stack deploy",
					},
				},
				Build: Build{},
			},
			isErrorExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := Read(tt.manifest)
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			assert.Equal(t, tt.expected, manifest)
		})
	}
}

func TestDeployInfoUnmarshalling(t *testing.T) {
	tests := []struct {
		name               string
		deployInfoManifest []byte
		expected           *Deploy
		isErrorExpected    bool
	}{
		{
			name: "list of commands",
			deployInfoManifest: []byte(`
- okteto stack deploy`),
			expected: &Deploy{
				Commands: []string{
					"okteto stack deploy",
				},
			},
		},
		{
			name: "commands",
			deployInfoManifest: []byte(`commands:
- okteto stack deploy`),
			expected: &Deploy{
				Commands: []string{},
			},
			isErrorExpected: true,
		},
		{
			name: "compose with endpoints",
			deployInfoManifest: []byte(`compose:
  manifest: path
  endpoints:
    - path: /
      service: app
      port: 80`),
			expected: &Deploy{
				Commands: []string{},
			},
			isErrorExpected: true,
		},
		{
			name: "divert",
			deployInfoManifest: []byte(`divert:
  from:
    namespace: staging
    ingress: movies
    service: frontend
    deployment: frontend
  to:
    service: frontend`),
			expected: &Deploy{
				Commands: []string{},
			},
			isErrorExpected: true,
		},
		{
			name: "all together",
			deployInfoManifest: []byte(`commands:
- kubectl apply -f manifest.yml
divert:
  from:
    namespace: staging
    ingress: movies
    service: frontend
    deployment: frontend
  to:
    service: frontend
compose:
  manifest: ./docker-compose.yml
  endpoints:
  - path: /
    service: frontend
    port: 80
  - path: /api
    service: api
    port: 8080`),
			expected: &Deploy{
				Commands: []string{},
			},
			isErrorExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewDeploy()

			err := yaml.UnmarshalStrict(tt.deployInfoManifest, &result)
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManifestBuildUnmarshalling(t *testing.T) {
	tests := []struct {
		name            string
		buildManifest   []byte
		expected        Build
		isErrorExpected bool
	}{
		{
			name:          "unmarshalling-relative-path",
			buildManifest: []byte(`service1: ./service1`),
			expected: Build{
				"service1": {
					Name:    "./service1",
					Context: "",
				},
			},
		},
		{
			name: "unmarshalling-all-fields",
			buildManifest: []byte(`service2:
  image: image-tag
  context: ./service2
  dockerfile: Dockerfile
  args:
    key1: value1
  cache_from:
    - cache-image`),
			expected: Build{
				"service2": {
					Context:    "./service2",
					Dockerfile: "Dockerfile",
					Image:      "image-tag",
					Args: []environment.EnvVar{
						{
							Name:  "key1",
							Value: "value1",
						},
					},
					CacheFrom: []string{"cache-image"},
				},
			},
		},
		{
			name: "invalid-fields",
			buildManifest: []byte(`service1:
  file: Dockerfile`),
			expected:        Build{},
			isErrorExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Build
			err := yaml.UnmarshalStrict(tt.buildManifest, &result)
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}
