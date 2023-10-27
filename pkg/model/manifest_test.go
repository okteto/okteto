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
	"path/filepath"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/discovery"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			name: "expand image for remote deploy",
			envs: map[string]string{
				"myImage": "test",
			},
			manifest: &Manifest{
				Deploy: &DeployInfo{
					Image: "${myImage}",
				},
			},
			expectedManifest: &Manifest{
				Deploy: &DeployInfo{
					Image: "test",
				},
			},
		},
		{
			name: "does not expand vars in destroy command",
			envs: map[string]string{
				"TEST_VAR": "test",
			},
			manifest: &Manifest{
				Destroy: &DestroyInfo{
					Image: "",
					Commands: []DeployCommand{
						{
							Name: "test",
							Command: `TEST_VAR="do-not-expand-me"
echo $TEST_VAR`,
						},
					},
				},
			},
			expectedManifest: &Manifest{
				Destroy: &DestroyInfo{
					Image: "",
					Commands: []DeployCommand{
						{
							Name: "test",
							Command: `TEST_VAR="do-not-expand-me"
echo $TEST_VAR`,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
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
				t.Setenv(k, v)
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
				Driver:               constants.OktetoDivertWeaverDriver,
				Namespace:            "namespace",
				DeprecatedService:    "service",
				DeprecatedPort:       8080,
				DeprecatedDeployment: "deployment",
			},
			expectedErr: nil,
		},
		{
			name: "divert-ok-without-service",
			divert: DivertDeploy{
				Driver:               constants.OktetoDivertWeaverDriver,
				Namespace:            "namespace",
				DeprecatedService:    "",
				DeprecatedPort:       8080,
				DeprecatedDeployment: "deployment",
			},
			expectedErr: nil,
		},
		{
			name: "divert-ok-without-deployment",
			divert: DivertDeploy{
				Driver:               constants.OktetoDivertWeaverDriver,
				Namespace:            "namespace",
				DeprecatedService:    "service",
				DeprecatedPort:       8080,
				DeprecatedDeployment: "",
			},
			expectedErr: nil,
		},
		{
			name: "divert-ok-without-port",
			divert: DivertDeploy{
				Driver:               constants.OktetoDivertWeaverDriver,
				Namespace:            "namespace",
				DeprecatedService:    "service",
				DeprecatedDeployment: "deployment",
			},
			expectedErr: nil,
		},
		{
			name: "divert-ko-without-namespace",
			divert: DivertDeploy{
				Driver:               constants.OktetoDivertWeaverDriver,
				Namespace:            "",
				DeprecatedService:    "service",
				DeprecatedPort:       8080,
				DeprecatedDeployment: "deployment",
			},
			expectedErr: fmt.Errorf("the field 'deploy.divert.namespace' is mandatory"),
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
	tests := []struct {
		name         string
		buildSection ManifestBuild
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
	devInterface := Localhost
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
					Image: constants.OktetoPipelineRunnerImage,
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
					Image: constants.OktetoPipelineRunnerImage,
					ComposeSection: &ComposeSectionInfo{
						Stack: stack,
					},
				},
				Destroy: &DestroyInfo{},
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
					Image: constants.OktetoPipelineRunnerImage,
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
				Dev:     ManifestDevs{},
				Destroy: &DestroyInfo{},
				Deploy: &DeployInfo{
					Image: constants.OktetoPipelineRunnerImage,
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
				Destroy: &DestroyInfo{},
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
						Mode: "sync",
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
	t.Setenv("my_key", "my_value")
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

func TestHasDependencies(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		expected bool
	}{
		{
			name: "nil dependencies",
			manifest: Manifest{
				Dependencies: nil,
			},
			expected: false,
		},
		{
			name: "empty dependencies",
			manifest: Manifest{
				Dependencies: make(ManifestDependencies, 0),
			},
			expected: false,
		},
		{
			name: "has dependencies",
			manifest: Manifest{
				Dependencies: ManifestDependencies{
					"test": &Dependency{},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.manifest.HasDependencies())
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

func Test_getManifestFromFile(t *testing.T) {
	tests := []struct {
		name          string
		manifestBytes []byte
		composeBytes  []byte
		expectedErr   error
	}{
		{
			name:          "manifestPath to a valid compose file",
			manifestBytes: nil,
			composeBytes: []byte(`services:
  test:
    image: test`),
		},
		{
			name:          "manifestPath to a invalid compose file with empty service",
			manifestBytes: nil,
			composeBytes: []byte(`services:
  test:
          `),
			expectedErr: oktetoErrors.ErrServiceEmpty,
		},
		{
			name:          "manifestPath to empty okteto manifest, no compose file",
			manifestBytes: []byte(``),
			composeBytes:  nil,
			expectedErr:   oktetoErrors.ErrEmptyManifest,
		},
		{
			name:          "manifestPath to invalid okteto manifest, no compose file",
			manifestBytes: []byte(`asdasa: asda`),
			composeBytes:  nil,
			expectedErr:   oktetoErrors.ErrInvalidManifest,
		},
		{
			name: "manifestPath to valid v2 okteto manifest",
			manifestBytes: []byte(`dev:
  api:
    sync:
      - .:/usr`),
			composeBytes: nil,
		},
		{
			name: "manifestPath to valid v1 okteto manifest",
			manifestBytes: []byte(`name: test
sync:
  - .:/usr`),
			composeBytes: nil,
		},
		{
			name:          "manifestPath to not existent okteto manifest, no compose file",
			manifestBytes: nil,
			composeBytes:  nil,
			expectedErr:   discovery.ErrOktetoManifestNotFound,
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

			assert.ErrorIs(t, err, tt.expectedErr)
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

func Test_GetTimeout(t *testing.T) {
	tests := []struct {
		name           string
		defaultTimeout time.Duration
		dependency     *Dependency
		expected       time.Duration
	}{
		{
			name:           "default timeout set and specific not",
			defaultTimeout: 5 * time.Minute,
			dependency:     &Dependency{},
			expected:       5 * time.Minute,
		},
		{
			name: "default timeout unset and specific set",
			dependency: &Dependency{
				Timeout: 10 * time.Minute,
			},
			expected: 10 * time.Minute,
		},
		{
			name:       "both unset",
			dependency: &Dependency{},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dependency.GetTimeout(tt.defaultTimeout)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ExpandVars(t *testing.T) {
	t.Setenv("MY_CUSTOM_VAR_FROM_ENVIRON", "varValueFromEnv")
	dependency := Dependency{
		Repository:   "${REPO}",
		Branch:       "${NOBRANCHSET-$BRANCH}",
		ManifestPath: "${NOMPATHSET=$MPATH}",
		Namespace:    "${FOO+$SOME_NS_DEP_EXP}",
		Variables: Environment{
			EnvVar{
				Name:  "MYVAR",
				Value: "${AVARVALUE}",
			},
			EnvVar{
				Name:  "$${ANAME}",
				Value: "${MY_CUSTOM_VAR_FROM_ENVIRON}",
			},
		},
	}
	expected := Dependency{
		Repository:   "my/repo",
		Branch:       "myBranch",
		ManifestPath: "api/okteto.yml",
		Namespace:    "oktetoNs",
		Variables: Environment{
			EnvVar{
				Name:  "MYVAR",
				Value: "thisIsAValue",
			},
			EnvVar{
				Name:  "${ANAME}",
				Value: "varValueFromEnv",
			},
		},
	}
	envVariables := []string{
		"FOO=BAR",
		"REPO=my/repo",
		"BRANCH=myBranch",
		"MPATH=api/okteto.yml",
		"SOME_NS_DEP_EXP=oktetoNs",
		"AVARVALUE=thisIsAValue",
	}

	err := dependency.ExpandVars(envVariables)
	require.NoError(t, err)
	assert.Equal(t, expected, dependency)
}

func Test_Manifest_HasDeploySection(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		expected bool
	}{
		{
			name:     "nil manifest",
			expected: false,
		},
		{
			name:     "m.IsV2 is false",
			manifest: &Manifest{},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Deploy is nil",
			manifest: &Manifest{
				IsV2: true,
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Deploy.Commands is nil",
			manifest: &Manifest{
				IsV2:   true,
				Deploy: &DeployInfo{},
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Deploy.Commands is empty",
			manifest: &Manifest{
				IsV2: true,
				Deploy: &DeployInfo{
					Commands: []DeployCommand{},
				},
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Deploy.Commands has items",
			manifest: &Manifest{
				IsV2: true,
				Deploy: &DeployInfo{
					Commands: []DeployCommand{
						{
							Name:    "test",
							Command: "echo test",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "m.IsV2 && m.Deploy.ComposeSection is nil",
			manifest: &Manifest{
				IsV2:   true,
				Deploy: &DeployInfo{},
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Deploy.ComposeSection.ComposesInfo is nil",
			manifest: &Manifest{
				IsV2: true,
				Deploy: &DeployInfo{
					ComposeSection: &ComposeSectionInfo{},
				},
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Deploy.ComposeSection.ComposesInfo has items",
			manifest: &Manifest{
				IsV2: true,
				Deploy: &DeployInfo{
					ComposeSection: &ComposeSectionInfo{
						ComposesInfo: ComposeInfoList{
							{
								File:             "docker-compose.yml",
								ServicesToDeploy: ServicesToDeploy{"test"},
							},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.HasDeploySection()
			assert.Equal(t, tt.expected, got)
		})

	}
}

func Test_Manifest_HasDependenciesSection(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		expected bool
	}{
		{
			name:     "nil manifest",
			expected: false,
		},
		{
			name:     "m.IsV2 is false",
			manifest: &Manifest{},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Dependencies is nil",
			manifest: &Manifest{
				IsV2: true,
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Dependencies has items",
			manifest: &Manifest{
				IsV2: true,
				Dependencies: ManifestDependencies{
					"test": &Dependency{},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.HasDependenciesSection()
			assert.Equal(t, tt.expected, got)
		})

	}
}

func Test_Manifest_HasBuildSection(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		expected bool
	}{
		{
			name:     "nil manifest",
			expected: false,
		},
		{
			name:     "m.IsV2 is false",
			manifest: &Manifest{},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Build is nil",
			manifest: &Manifest{
				IsV2: true,
			},
			expected: false,
		},
		{
			name: "m.IsV2 && m.Build has items",
			manifest: &Manifest{
				IsV2: true,
				Build: ManifestBuild{
					"test": &BuildInfo{},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.HasBuildSection()
			assert.Equal(t, tt.expected, got)
		})

	}
}

func Test_getInferredManifestFromK8sManifestFile(t *testing.T) {
	wd := t.TempDir()
	fullpath := filepath.Join(wd, "k8s.yml")
	f, err := os.Create(fullpath)
	assert.NoError(t, err)
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("Error closing file %s: %s", fullpath, err)
		}
	}()
	_, err = GetInferredManifest(wd)
	assert.NoError(t, err)
}

func Test_getInferredManifestFromK8sManifestFolder(t *testing.T) {
	wd := t.TempDir()
	fullpath := filepath.Join(wd, "manifests")
	assert.NoError(t, os.MkdirAll(filepath.Dir(fullpath), 0750))
	f, err := os.Create(fullpath)
	assert.NoError(t, err)
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("Error closing file %s: %s", fullpath, err)
		}
	}()

	_, err = GetInferredManifest(wd)
	assert.NoError(t, err)
}

func Test_getInferredManifestFromHelmPath(t *testing.T) {
	var tests = []struct {
		name          string
		filesToCreate []string
		expected      string
	}{
		{
			name:          "chart folder exists on wd",
			filesToCreate: []string{filepath.Join("chart", "Chart.yaml")},
			expected:      "charts",
		},
		{
			name:          "chart folder inside helm folder exists on wd",
			filesToCreate: []string{filepath.Join("helm", "charts", "Chart.yaml")},
			expected:      filepath.Join("helm", "charts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			for _, fileToCreate := range tt.filesToCreate {
				fullpath := filepath.Join(wd, fileToCreate)
				assert.NoError(t, os.MkdirAll(filepath.Dir(fullpath), 0750))
				f, err := os.Create(fullpath)
				assert.NoError(t, err)
				defer func() {
					if err := f.Close(); err != nil {
						t.Fatalf("Error closing file %s: %s", fullpath, err)
					}
				}()
			}
			_, err := GetInferredManifest(wd)
			assert.NoError(t, err)
		})
	}
}

func Test_getInferredManifestWhenNoManifestExist(t *testing.T) {
	wd := t.TempDir()
	result, err := GetInferredManifest(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, oktetoErrors.ErrCouldNotInferAnyManifest)
}
