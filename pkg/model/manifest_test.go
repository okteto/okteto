// Copyright 2022 The Okteto Authors
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
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestManifestExpandEnvs(t *testing.T) {
	tests := []struct {
		name            string
		envs            map[string]string
		manifest        []byte
		expectedErr     bool
		expectedCommand string
	}{
		{
			name: "expand envs on command",
			envs: map[string]string{
				"OKTETO_GIT_COMMIT": "dev",
			},
			manifest: []byte(`icon: https://apps.okteto.com/movies/icon.png
deploy:
  - okteto build -t okteto.dev/api:${OKTETO_GIT_COMMIT} api
  - okteto build -t okteto.dev/frontend:${OKTETO_GIT_COMMIT} frontend
  - helm upgrade --install movies chart --set tag=${OKTETO_GIT_COMMIT}
devs:
  - api/okteto.yml
  - frontend/okteto.yml`),
			expectedCommand: "okteto build -t okteto.dev/api:dev api",
		},
		{
			name: "expand envs on command without env var set",
			envs: map[string]string{},
			manifest: []byte(`icon: https://apps.okteto.com/movies/icon.png
deploy:
  - okteto build -t okteto.dev/api:${OKTETO_GIT_COMMIT:=dev} api
  - okteto build -t okteto.dev/frontend:${OKTETO_GIT_COMMIT} frontend
  - helm upgrade --install movies chart --set tag=${OKTETO_GIT_COMMIT}
devs:
  - api/okteto.yml
  - frontend/okteto.yml`),
			expectedCommand: "okteto build -t okteto.dev/api:dev api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				os.Setenv(k, v)
			}
			m, err := Read(tt.manifest)
			assert.NoError(t, err)

			m, err = m.ExpandEnvVars()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				assert.Equal(t, tt.expectedCommand, m.Deploy.Commands[0].Command)
			}

		})
	}
}

func TestInferFromStack(t *testing.T) {
	devInterface := PrivilegedLocalhost
	if runtime.GOOS == "windows" {
		devInterface = Localhost
	}
	stack := &Stack{
		Services: map[string]*Service{
			"test": {
				Build: &BuildInfo{
					Name:       "test",
					Context:    "test",
					Dockerfile: "test/Dockerfile",
				},
				Ports: []Port{
					{
						HostPort:      8080,
						ContainerPort: 8080,
					},
				},
			},
		},
	}
	tests := []struct {
		name             string
		currentManifest  *Manifest
		expectedManifest *Manifest
	}{
		{
			name: "infer from stack empty dev",
			currentManifest: &Manifest{
				Dev:   ManifestDevs{},
				Build: ManifestBuild{},
				Deploy: &DeployInfo{
					Compose: &ComposeInfo{
						Stack: stack,
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:          "test",
						Dockerfile:       "test/Dockerfile",
						VolumesToInclude: []StackVolume{},
					},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Name: "test",
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Selector: Selector{},
						Forward: []Forward{
							{
								Local:  8080,
								Remote: 8080,
							},
						},
						EmptyImage: true,
						Image: &BuildInfo{
							Context:    ".",
							Dockerfile: "Dockerfile",
						},
						Push: &BuildInfo{
							Context:    ".",
							Dockerfile: "Dockerfile",
						},
						ImagePullPolicy: apiv1.PullAlways,
						Probes:          &Probes{},
						Lifecycle:       &Lifecycle{},
						Workdir:         "/okteto",
						SecurityContext: &SecurityContext{
							RunAsUser:  pointer.Int64(0),
							RunAsGroup: pointer.Int64(0),
							FSGroup:    pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Volumes:       []Volume{},
						Timeout: Timeout{
							Default:   60 * time.Second,
							Resources: 120 * time.Second,
						},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Secrets: []Secret{},
						Command: Command{
							Values: []string{"sh"},
						},
						Interface: devInterface,
						Services:  []*Dev{},
						Sync: Sync{
							RescanInterval: 300,
							Folders: []SyncFolder{
								{
									LocalPath:  ".",
									RemotePath: "/okteto",
								},
							},
						},
					},
				},
				Deploy: &DeployInfo{
					Compose: &ComposeInfo{
						Stack: stack,
					},
				},
			},
		},
		{
			name: "infer from stack not overriding build",
			currentManifest: &Manifest{
				Dev: ManifestDevs{},
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:    "test-1",
						Dockerfile: "test-1/Dockerfile",
					},
				},
				Deploy: &DeployInfo{
					Compose: &ComposeInfo{
						Stack: stack,
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:    "test-1",
						Dockerfile: "test-1/Dockerfile",
					},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Name: "test",
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Selector: Selector{},
						Forward: []Forward{
							{
								Local:  8080,
								Remote: 8080,
							},
						},
						EmptyImage: true,
						Image: &BuildInfo{
							Context:    ".",
							Dockerfile: "Dockerfile",
						},
						Push: &BuildInfo{
							Context:    ".",
							Dockerfile: "Dockerfile",
						},
						ImagePullPolicy: apiv1.PullAlways,
						Probes:          &Probes{},
						Lifecycle:       &Lifecycle{},
						Workdir:         "/okteto",
						SecurityContext: &SecurityContext{
							RunAsUser:  pointer.Int64(0),
							RunAsGroup: pointer.Int64(0),
							FSGroup:    pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Volumes:       []Volume{},
						Timeout: Timeout{
							Default:   60 * time.Second,
							Resources: 120 * time.Second,
						},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Secrets: []Secret{},
						Command: Command{
							Values: []string{"sh"},
						},
						Interface: "0.0.0.0",
						Services:  []*Dev{},
						Sync: Sync{
							RescanInterval: 300,
							Folders: []SyncFolder{
								{
									LocalPath:  ".",
									RemotePath: "/okteto",
								},
							},
						},
					},
				},
				Deploy: &DeployInfo{
					Compose: &ComposeInfo{
						Stack: stack,
					},
				},
			},
		},
		{
			name: "infer from stack not overriding dev",
			currentManifest: &Manifest{
				Dev: ManifestDevs{
					"test": &Dev{
						Name:      "one",
						Namespace: "test",
					},
				},
				Build: ManifestBuild{},
				Deploy: &DeployInfo{
					Compose: &ComposeInfo{
						Stack: stack,
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:          "test",
						Dockerfile:       "test/Dockerfile",
						VolumesToInclude: []StackVolume{},
					},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Name:      "one",
						Namespace: "test",
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Selector:   Selector{},
						EmptyImage: true,
						Image: &BuildInfo{
							Context:    ".",
							Dockerfile: "Dockerfile",
						},
						ImagePullPolicy: apiv1.PullAlways,
						Probes:          &Probes{},
						Lifecycle:       &Lifecycle{},
						Workdir:         "/okteto",
						SecurityContext: &SecurityContext{
							RunAsUser:  pointer.Int64(0),
							RunAsGroup: pointer.Int64(0),
							FSGroup:    pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Volumes:       []Volume{},
						Timeout: Timeout{
							Default:   60 * time.Second,
							Resources: 120 * time.Second,
						},
						Command: Command{
							Values: []string{"sh"},
						},
						Interface: "0.0.0.0",
						Sync: Sync{
							RescanInterval: 300,
							Folders: []SyncFolder{
								{
									LocalPath:  ".",
									RemotePath: "/okteto",
								},
							},
						},
					},
				},
				Deploy: &DeployInfo{
					Compose: &ComposeInfo{
						Stack: stack,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.currentManifest.InferFromStack()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedManifest, result)
		})
	}
}
