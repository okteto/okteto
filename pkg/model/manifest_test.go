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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestManifestExpandDevEnvs(t *testing.T) {
	tests := []struct {
		name             string
		envs             map[string]string
		manifest         *Manifest
		expectedManifest *Manifest
	}{
		{
			name: "autocreate without image but build section defined",
			envs: map[string]string{
				"OKTETO_BUILD_TEST_IMAGE": "test",
			},
			manifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Autocreate: true,
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Autocreate: true,
						Image: &BuildInfo{
							Name: "test",
						},
					},
				},
			},
		},
		{
			name:             "nothing to expand",
			manifest:         &Manifest{},
			expectedManifest: &Manifest{},
		},

		{
			name: "autocreate with image and build section defined",
			envs: map[string]string{
				"build":   "test",
				"myImage": "test-2",
			},
			manifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Autocreate: true,
						Image: &BuildInfo{
							Name: "${myImage}",
						},
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"test": &Dev{
						Autocreate: true,
						Image: &BuildInfo{
							Name: "test-2",
						},
					},
				},
			},
		},
		{
			name: "autocreate with image",
			envs: map[string]string{
				"build": "test",
			},
			manifest: &Manifest{
				Dev: ManifestDevs{
					"test": &Dev{
						Autocreate: true,
						Image: &BuildInfo{
							Name: "${build}",
						},
					},
				},
			},
			expectedManifest: &Manifest{
				Dev: ManifestDevs{
					"test": &Dev{
						Autocreate: true,
						Image: &BuildInfo{
							Name: "test",
						},
					},
				},
			},
		},
		{
			name: "expand image",
			envs: map[string]string{
				"build": "test",
			},
			manifest:         &Manifest{},
			expectedManifest: &Manifest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				os.Setenv(k, v)
			}

			err := tt.manifest.ExpandEnvVars()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedManifest, tt.manifest)
		})
	}
}
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
			expectedCommand: "okteto build -t okteto.dev/api:${OKTETO_GIT_COMMIT} api",
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
			expectedCommand: "okteto build -t okteto.dev/api:${OKTETO_GIT_COMMIT:=dev} api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				os.Setenv(k, v)
			}
			m, err := Read(tt.manifest)
			assert.NoError(t, err)

			err = m.ExpandEnvVars()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCommand, m.Deploy.Commands[0].Command)
			}

		})
	}
}

func Test_validateDivert(t *testing.T) {
	tests := []struct {
		name        string
		divert      DivertDeploy
		expectedErr error
	}{
		{
			name: "divert-ok-with-port",
			divert: DivertDeploy{
				Namespace:  "namespace",
				Service:    "service",
				Port:       8080,
				Deployment: "deployment",
			},
			expectedErr: nil,
		},
		{
			name: "divert-ok-without-port",
			divert: DivertDeploy{
				Namespace:  "namespace",
				Service:    "service",
				Deployment: "deployment",
			},
			expectedErr: nil,
		},
		{
			name: "divert-ko-without-namespace",
			divert: DivertDeploy{
				Namespace:  "",
				Service:    "service",
				Port:       8080,
				Deployment: "deployment",
			},
			expectedErr: fmt.Errorf("the field 'deploy.divert.namespace' is mandatory"),
		},
		{
			name: "divert-ko-without-service",
			divert: DivertDeploy{
				Namespace:  "namespace",
				Service:    "",
				Port:       8080,
				Deployment: "deployment",
			},
			expectedErr: fmt.Errorf("the field 'deploy.divert.service' is mandatory"),
		},
		{
			name: "divert-ko-without-deployment",
			divert: DivertDeploy{
				Namespace:  "namespace",
				Service:    "service",
				Port:       8080,
				Deployment: "",
			},
			expectedErr: fmt.Errorf("the field 'deploy.divert.deployment' is mandatory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manifest{
				Deploy: &DeployInfo{
					Divert: &tt.divert,
				},
			}
			assert.Equal(t, m.validateDivert(), tt.expectedErr)
		})
	}
}

func Test_validateManifestBuild(t *testing.T) {
	dir := t.TempDir()
	tmpTestSecretFile, err := os.CreateTemp(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = tmpTestSecretFile.Close()
	}()
	tests := []struct {
		name         string
		buildSection ManifestBuild
		dependencies ManifestDependencies
		expectedErr  bool
	}{
		{
			name: "no cycle - no connections",
			buildSection: ManifestBuild{
				"a": &BuildInfo{},
				"b": &BuildInfo{},
				"c": &BuildInfo{},
			},
			expectedErr: false,
		},
		{
			name: "no cycle - connections",
			buildSection: ManifestBuild{
				"a": &BuildInfo{
					DependsOn: []string{"b"},
				},
				"b": &BuildInfo{
					DependsOn: []string{"c"},
				},
				"c": &BuildInfo{},
			},
			expectedErr: false,
		},
		{
			name: "cycle - same node dependency",
			buildSection: ManifestBuild{
				"a": &BuildInfo{
					DependsOn: []string{"a"},
				},
				"b": &BuildInfo{
					DependsOn: []string{},
				},
				"c": &BuildInfo{},
			},
			expectedErr: true,
		},
		{
			name: "cycle - direct cycle",
			buildSection: ManifestBuild{
				"a": &BuildInfo{
					DependsOn: []string{"b"},
				},
				"b": &BuildInfo{
					DependsOn: []string{"a"},
				},
				"c": &BuildInfo{},
			},
			expectedErr: true,
		},
		{
			name: "cycle - indirect cycle",
			buildSection: ManifestBuild{
				"a": &BuildInfo{
					DependsOn: []string{"b"},
				},
				"b": &BuildInfo{
					DependsOn: []string{"c"},
				},
				"c": &BuildInfo{
					DependsOn: []string{"a"},
				},
			},
			expectedErr: true,
		},
		{
			name: "local dependency pointing to folder - error",
			dependencies: ManifestDependencies{
				"test": LocalDependency{
					manifestPath: tmpTestSecretFile.Name(),
				},
			},
		},
		{
			name: "local dependency pointing to non existen path - error",
			dependencies: ManifestDependencies{
				"test": LocalDependency{
					manifestPath: filepath.Clean("/test/test/test"),
				},
			},
		},
		{
			name: "local dependency pointing to correct file - no error",
			dependencies: ManifestDependencies{
				"test": LocalDependency{
					manifestPath: tmpTestSecretFile.Name(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manifest{
				Build: tt.buildSection,
			}
			assert.Equal(t, tt.expectedErr, m.validate() != nil)
		})
	}
}

func TestInferFromStack(t *testing.T) {
	dirtest := filepath.Clean("/stack/dir/")
	devInterface := PrivilegedLocalhost
	if runtime.GOOS == "windows" {
		devInterface = Localhost
	}
	stack := &Stack{
		Services: map[string]*Service{
			"test": {
				Build: &BuildInfo{
					Name:       "",
					Context:    "test",
					Dockerfile: "Dockerfile",
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
					ComposeSection: &ComposeSectionInfo{
						Stack: &Stack{
							Services: map[string]*Service{
								"test": {
									Build: &BuildInfo{
										Name:       "test",
										Context:    filepath.Join(dirtest, "test"),
										Dockerfile: filepath.Join(filepath.Join(dirtest, "test"), "Dockerfile"),
									},
									Ports: []Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:    "test",
						Dockerfile: "Dockerfile",
					},
				},
				Dev: ManifestDevs{},
				Deploy: &DeployInfo{
					ComposeSection: &ComposeSectionInfo{
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
						Dockerfile: filepath.Join("test-1", "Dockerfile"),
					},
				},
				Deploy: &DeployInfo{
					ComposeSection: &ComposeSectionInfo{
						Stack: &Stack{
							Services: map[string]*Service{
								"test": {
									Build: &BuildInfo{
										Name:       "test",
										Context:    filepath.Join(dirtest, "test"),
										Dockerfile: filepath.Join(filepath.Join(dirtest, "test"), "Dockerfile"),
									},
									Ports: []Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:    "test-1",
						Dockerfile: filepath.Join("test-1", "Dockerfile"),
					},
				},
				Dev: ManifestDevs{},
				Deploy: &DeployInfo{
					ComposeSection: &ComposeSectionInfo{
						Stack: &Stack{
							Services: map[string]*Service{
								"test": {
									Build: &BuildInfo{
										Name:       "test",
										Context:    "test",
										Dockerfile: "Dockerfile",
									},
									Ports: []Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
									},
								},
							},
						},
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
					ComposeSection: &ComposeSectionInfo{
						Stack: &Stack{
							Services: map[string]*Service{
								"test": {
									Build: &BuildInfo{
										Name:       "test",
										Context:    "test",
										Dockerfile: "Dockerfile",
									},
									Ports: []Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"test": &BuildInfo{
						Context:    "test",
						Dockerfile: "Dockerfile",
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
						Push: &BuildInfo{
							Context:    ".",
							Dockerfile: "Dockerfile",
						},
						ImagePullPolicy: apiv1.PullAlways,
						InitContainer:   InitContainer{Image: OktetoBinImageTag},
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
						Interface: devInterface,
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
					ComposeSection: &ComposeSectionInfo{
						Stack: stack,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.currentManifest.InferFromStack(filepath.Clean(dirtest))
			if result != nil {
				for _, d := range result.Dev {
					d.parentSyncFolder = ""
				}
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedManifest, result)
		})
	}
}

func TestSetManifestDefaultsFromDev(t *testing.T) {
	os.Setenv("my_key", "my_value")
	tests := []struct {
		name              string
		currentManifest   *Manifest
		expectedContext   string
		expectedNamespace string
	}{
		{
			name: "setting only manifest.Namespace",
			currentManifest: &Manifest{
				Dev: ManifestDevs{
					"test": &Dev{
						Namespace: "other-ns",
					},
				},
			},
			expectedContext:   "",
			expectedNamespace: "other-ns",
		},
		{
			name: "setting only manifest.Context",
			currentManifest: &Manifest{
				Dev: ManifestDevs{
					"test": &Dev{
						Context: "other-ctx",
					},
				},
			},
			expectedContext:   "other-ctx",
			expectedNamespace: "",
		},
		{
			name: "setting manifest.Context & manifest.Namespace",
			currentManifest: &Manifest{
				Dev: ManifestDevs{
					"test": &Dev{
						Context:   "other-ctx",
						Namespace: "other-ns",
					},
				},
			},
			expectedContext:   "other-ctx",
			expectedNamespace: "other-ns",
		},
		{
			name: "not overwrite if manifest has more than one dev",
			currentManifest: &Manifest{
				Namespace: "test",
				Context:   "test",
				Dev: ManifestDevs{
					"test": &Dev{
						Context: "other-ctx",
					},
					"test-2": &Dev{
						Context: "other-ctx",
					},
				},
			},
			expectedContext:   "test",
			expectedNamespace: "test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.currentManifest.setManifestDefaultsFromDev()
			assert.Equal(t, tt.expectedContext, tt.currentManifest.Context)
			assert.Equal(t, tt.expectedNamespace, tt.currentManifest.Namespace)
		})
	}
}

func TestSetBuildDefaults(t *testing.T) {

	tests := []struct {
		name              string
		currentBuildInfo  BuildInfo
		expectedBuildInfo BuildInfo
	}{
		{
			name:             "all empty",
			currentBuildInfo: BuildInfo{},
			expectedBuildInfo: BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
		},
		{
			name: "context empty",
			currentBuildInfo: BuildInfo{
				Dockerfile: "Dockerfile",
			},
			expectedBuildInfo: BuildInfo{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
		},
		{
			name: "dockerfile empty",
			currentBuildInfo: BuildInfo{
				Context: "buildName",
			},
			expectedBuildInfo: BuildInfo{
				Context:    "buildName",
				Dockerfile: "Dockerfile",
			},
		},
		{
			name: "context and Dockerfile filled",
			currentBuildInfo: BuildInfo{
				Context:    "buildName",
				Dockerfile: "Dockerfile",
			},
			expectedBuildInfo: BuildInfo{
				Context:    "buildName",
				Dockerfile: "Dockerfile",
			},
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			tt.currentBuildInfo.setBuildDefaults()

			assert.Equal(t, tt.expectedBuildInfo, tt.currentBuildInfo)
		})
	}
}

func TestGetManifestFromFile(t *testing.T) {
	tests := []struct {
		name          string
		manifestBytes []byte
		composeBytes  []byte
		expectedErr   bool
	}{
		{
			name:          "OktetoManifest does not exist and compose manifest is correct",
			manifestBytes: nil,
			composeBytes: []byte(`services:
  test:
    image: test`),
			expectedErr: false,
		},
		{
			name:          "OktetoManifest not contains any content and compose manifest does not exists",
			manifestBytes: []byte(``),
			composeBytes:  nil,
			expectedErr:   true,
		},
		{
			name:          "OktetoManifest is invalid and compose manifest does not exists",
			manifestBytes: []byte(`asdasa: asda`),
			composeBytes:  nil,
			expectedErr:   true,
		},
		{
			name: "OktetoManifestV2 is ok",
			manifestBytes: []byte(`dev:
  api:
    sync:
    - .:/usr`),
			composeBytes: nil,
			expectedErr:  false,
		},
		{
			name: "OktetoManifestV1 is ok",
			manifestBytes: []byte(`name: test
sync:
- .:/usr`),
			composeBytes: nil,
			expectedErr:  false,
		},
		{
			name:          "OktetoManifest and compose manifest does not exists",
			manifestBytes: nil,
			composeBytes:  nil,
			expectedErr:   true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := ""
			if tt.manifestBytes != nil {
				file = filepath.Join(dir, "okteto.yml")
				assert.NoError(t, os.WriteFile(filepath.Join(dir, "okteto.yml"), tt.manifestBytes, 0600))
			}
			if tt.composeBytes != nil {
				if file == "" {
					file = filepath.Join(dir, "docker-compose.yml")
				}
				assert.NoError(t, os.WriteFile(filepath.Join(dir, "docker-compose.yml"), tt.composeBytes, 0600))
			}
			_, err := getManifestFromFile(dir, file)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestHasDev(t *testing.T) {
	tests := []struct {
		name       string
		devSection ManifestDevs
		devName    string
		out        bool
	}{
		{
			name: "devName is on dev section",
			devSection: ManifestDevs{
				"autocreate": &Dev{},
			},
			devName: "autocreate",
			out:     true,
		},
		{
			name: "devName is not on dev section",
			devSection: ManifestDevs{
				"autocreate": &Dev{},
			},
			devName: "not-autocreate",
			out:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.devSection.HasDev(tt.devName)
			assert.Equal(t, tt.out, result)
		})
	}
}

func Test_SanitizeSvcNames(t *testing.T) {
	tests := []struct {
		name             string
		manifest         *Manifest
		expectedManifest *Manifest
		expectedErr      error
	}{
		{
			name: "keys-have-uppercase",
			manifest: &Manifest{
				Build: ManifestBuild{
					"Frontend": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"Frontend": &Dev{},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "Frontend",
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"frontend": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"frontend": &Dev{
						Name: "frontend",
					},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "frontend",
					},
				},
			},
		},
		{
			name: "keys-have-spaces",
			manifest: &Manifest{
				Build: ManifestBuild{
					" my build service": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my dev service": &Dev{},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my global forward ",
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"my-build-service": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my-dev-service": &Dev{
						Name: "my-dev-service",
					},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my-global-forward",
					},
				},
			},
		},
		{
			name: "keys-have-underscore",
			manifest: &Manifest{
				Build: ManifestBuild{
					"my_build_service": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my_dev_service": &Dev{},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my_global_forward",
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"my-build-service": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my-dev-service": &Dev{
						Name: "my-dev-service",
					},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my-global-forward",
					},
				},
			},
		},
		{
			name: "keys-have-mix",
			manifest: &Manifest{
				Build: ManifestBuild{
					"  my_Build service": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my_DEV_service ": &Dev{},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my glOBal_forward",
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"my-build-service": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my-dev-service": &Dev{
						Name: "my-dev-service",
					},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my-global-forward",
					},
				},
			},
		},
		{
			name: "keys-have-trailing-spaces",
			manifest: &Manifest{
				Build: ManifestBuild{
					"  my-build ": &BuildInfo{},
				},
				Dev: ManifestDevs{
					" my-dev  ": &Dev{},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "   my-global   ",
					},
				},
			},
			expectedManifest: &Manifest{
				Build: ManifestBuild{
					"my-build": &BuildInfo{},
				},
				Dev: ManifestDevs{
					"my-dev": &Dev{
						Name: "my-dev",
					},
				},
				GlobalForward: []forward.GlobalForward{
					{
						ServiceName: "my-global",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.SanitizeSvcNames()
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedManifest, tt.manifest)
		})
	}
}

func TestManifestBuildMerge(t *testing.T) {
	tests := []struct {
		name                 string
		ogBuildSection       ManifestBuild
		otherBuildSection    ManifestBuild
		expectedBuildSection ManifestBuild
		expectedWarnings     []string
	}{
		{
			name: "different build section",
			ogBuildSection: ManifestBuild{
				"a": &BuildInfo{},
			},
			otherBuildSection: ManifestBuild{
				"b": &BuildInfo{},
			},
			expectedBuildSection: ManifestBuild{
				"a": &BuildInfo{},
				"b": &BuildInfo{},
			},
			expectedWarnings: []string{},
		},
		{
			name: "same build section",
			ogBuildSection: ManifestBuild{
				"a": &BuildInfo{},
			},
			otherBuildSection: ManifestBuild{
				"a": &BuildInfo{},
			},
			expectedBuildSection: ManifestBuild{
				"a": &BuildInfo{},
			},
			expectedWarnings: []string{
				"build.a: Build a already declared in the main manifest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.ogBuildSection.merge(tt.otherBuildSection)
			assert.Equal(t, tt.expectedBuildSection, tt.ogBuildSection)
			assert.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestManifestDependenciesMerge(t *testing.T) {
	tests := []struct {
		name                        string
		ogDependenciesSection       ManifestDependencies
		otherDependenciesSection    ManifestDependencies
		expectedDependenciesSection ManifestDependencies
		expectedWarnings            []string
	}{
		{
			name: "different dependencies section",
			ogDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			otherDependenciesSection: ManifestDependencies{
				"b": &RemoteDependency{},
			},
			expectedDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			expectedWarnings: []string{
				"dependencies: dependencies are only supported on the main manifest",
			},
		},
		{
			name: "same dependencies section",
			ogDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			otherDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			expectedDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			expectedWarnings: []string{
				"dependencies: dependencies are only supported on the main manifest",
			},
		},
		{
			name: "no other dependencies",
			ogDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			otherDependenciesSection: ManifestDependencies{},
			expectedDependenciesSection: ManifestDependencies{
				"a": &RemoteDependency{},
			},
			expectedWarnings: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.ogDependenciesSection.merge(tt.otherDependenciesSection)
			assert.Equal(t, tt.expectedDependenciesSection, tt.ogDependenciesSection)
			assert.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestManifestDeployMerge(t *testing.T) {
	tests := []struct {
		name                  string
		ogDeploySection       *DeployInfo
		otherDeploySection    *DeployInfo
		expectedDeploySection *DeployInfo
		expectedWarnings      []string
	}{
		{
			name: "merge commands",
			ogDeploySection: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    "test",
						Command: "test",
					},
				},
			},
			otherDeploySection: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    "test",
						Command: "test",
					},
				},
			},
			expectedDeploySection: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    "test",
						Command: "test",
					},
					{
						Name:    "test",
						Command: "test",
					},
				},
			},
			expectedWarnings: []string{},
		},
		{
			name:            "other has compose section",
			ogDeploySection: &DeployInfo{},
			otherDeploySection: &DeployInfo{
				ComposeSection: &ComposeSectionInfo{
					ComposesInfo: ComposeInfoList{
						ComposeInfo{
							File: "test",
						},
					},
				},
			},
			expectedDeploySection: &DeployInfo{},
			expectedWarnings: []string{
				"deploy.compose: compose can only be defined in main manifest",
			},
		},
		{
			name:            "other has endpoints section",
			ogDeploySection: &DeployInfo{},
			otherDeploySection: &DeployInfo{
				Endpoints: EndpointSpec{
					"aa": Endpoint{},
				},
			},
			expectedDeploySection: &DeployInfo{},
			expectedWarnings: []string{
				"deploy.endpoints: endpoints can only be defined in main manifest",
			},
		},
		{
			name:            "other has divert section",
			ogDeploySection: &DeployInfo{},
			otherDeploySection: &DeployInfo{
				Divert: &DivertDeploy{},
			},
			expectedDeploySection: &DeployInfo{},
			expectedWarnings: []string{
				"deploy.divert: divert can only be defined in main manifest",
			},
		},
		{
			name:            "multiple warnings",
			ogDeploySection: &DeployInfo{},
			otherDeploySection: &DeployInfo{
				ComposeSection: &ComposeSectionInfo{},
				Divert:         &DivertDeploy{},
			},
			expectedDeploySection: &DeployInfo{},
			expectedWarnings: []string{
				"deploy.compose: compose can only be defined in main manifest",
				"deploy.divert: divert can only be defined in main manifest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.ogDeploySection.merge(tt.otherDeploySection)
			assert.Equal(t, tt.expectedDeploySection, tt.ogDeploySection)
			assert.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestManifestDestroyMerge(t *testing.T) {
	tests := []struct {
		name                   string
		ogDestroySection       ManifestDestroy
		otherDestroySection    ManifestDestroy
		expectedDestroySection ManifestDestroy
	}{
		{
			name: "merge commands",
			ogDestroySection: ManifestDestroy{
				DeployCommand{
					Name:    "test",
					Command: "test",
				},
			},
			otherDestroySection: ManifestDestroy{
				DeployCommand{
					Name:    "test",
					Command: "test",
				},
			},
			expectedDestroySection: ManifestDestroy{
				DeployCommand{
					Name:    "test",
					Command: "test",
				},
				DeployCommand{
					Name:    "test",
					Command: "test",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.ogDestroySection.merge(tt.otherDestroySection)
			assert.Equal(t, tt.expectedDestroySection, tt.ogDestroySection)
		})
	}
}

func TestManifestDevMerge(t *testing.T) {
	tests := []struct {
		name               string
		ogDevSection       ManifestDevs
		otherDevSection    ManifestDevs
		expectedDevSection ManifestDevs
		expectedWarnings   []string
	}{
		{
			name: "different dev section",
			ogDevSection: ManifestDevs{
				"a": &Dev{},
			},
			otherDevSection: ManifestDevs{
				"b": &Dev{},
			},
			expectedDevSection: ManifestDevs{
				"a": &Dev{},
				"b": &Dev{},
			},
			expectedWarnings: []string{},
		},
		{
			name: "same dev section",
			ogDevSection: ManifestDevs{
				"a": &Dev{},
			},
			otherDevSection: ManifestDevs{
				"a": &Dev{},
			},
			expectedDevSection: ManifestDevs{
				"a": &Dev{},
			},
			expectedWarnings: []string{
				"dev.a: Dev a already declared in the main manifest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.ogDevSection.merge(tt.otherDevSection)
			assert.Equal(t, tt.expectedDevSection, tt.ogDevSection)
			assert.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestMergeWithOktetoManifest(t *testing.T) {
	tests := []struct {
		name             string
		manifest         *Manifest
		otherManifest    *Manifest
		expectedWarnings []string
	}{
		{
			name: "same context",
			manifest: &Manifest{
				Context: "a",
				Deploy:  &DeployInfo{},
			},
			otherManifest: &Manifest{
				Context: "a",
				Deploy:  &DeployInfo{},
			},
			expectedWarnings: []string{},
		},
		{
			name: "different context",
			manifest: &Manifest{
				Context: "a",
				Deploy:  &DeployInfo{},
			},
			otherManifest: &Manifest{
				Context: "b",
				Deploy:  &DeployInfo{},
			},
			expectedWarnings: []string{"context can only be defined in the main manifest"},
		},
		{
			name: "same icon",
			manifest: &Manifest{
				Icon:   "a",
				Deploy: &DeployInfo{},
			},
			otherManifest: &Manifest{
				Icon:   "a",
				Deploy: &DeployInfo{},
			},
			expectedWarnings: []string{},
		},
		{
			name: "different icon",
			manifest: &Manifest{
				Icon:   "a",
				Deploy: &DeployInfo{},
			},
			otherManifest: &Manifest{
				Icon:   "b",
				Deploy: &DeployInfo{},
			},
			expectedWarnings: []string{"icon can only be defined in the main manifest"},
		},
		{
			name: "same namespace",
			manifest: &Manifest{
				Namespace: "a",
				Deploy:    &DeployInfo{},
			},
			otherManifest: &Manifest{
				Namespace: "a",
				Deploy:    &DeployInfo{},
			},
			expectedWarnings: []string{},
		},
		{
			name: "different namespace",
			manifest: &Manifest{
				Namespace: "a",
				Deploy:    &DeployInfo{},
			},
			otherManifest: &Manifest{
				Namespace: "b",
				Deploy:    &DeployInfo{},
			},
			expectedWarnings: []string{"namespace can only be defined in the main manifest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.manifest.mergeWithOktetoManifest(tt.otherManifest)
			assert.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}
