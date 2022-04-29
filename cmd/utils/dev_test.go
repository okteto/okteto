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

package utils

import (
	"fmt"
	"os"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func Test_LoadManifestOrDefault(t *testing.T) {
	var tests = []struct {
		name       string
		deployment string
		expectErr  bool
		dev        *model.Dev
	}{
		{
			name:       "default",
			deployment: "default-deployment",
			expectErr:  false,
		},
		{
			name:       "default-no-name",
			deployment: "",
			expectErr:  true,
		},
		{
			name:       "load-dev",
			deployment: "test-deployment",
			expectErr:  false,
			dev: &model.Dev{
				Name: "loaded",
				Image: &model.BuildInfo{
					Name: "okteto/test:1.0",
				},
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  ".",
							RemotePath: "/path",
						},
					},
				},
			},
		},
	}

	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Name:      "test",
				Namespace: "namespace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := LoadManifestOrDefault("/tmp/a-path", tt.deployment)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error when loading")
				}

				if !oktetoErrors.IsNotExist(err) {
					t.Fatalf("expected not found got: %s", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if def.Dev[tt.deployment].Name != tt.deployment {
				t.Errorf("expected default name, got %s", tt.deployment)
			}

			if tt.dev == nil {
				return
			}

			f, err := os.CreateTemp("", "")
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			defer os.Remove(f.Name())

			if err := tt.dev.Save(f.Name()); err != nil {
				t.Fatal(err)
			}

			loaded, err := LoadManifestOrDefault(f.Name(), "foo")
			if err != nil {
				t.Fatalf("unexpected error when loading existing manifest: %s", err.Error())
			}

			if tt.dev.Image.Name != loaded.Dev["loaded"].Image.Name {
				t.Fatalf("expected %s got %s", tt.dev.Image.Name, loaded.Dev["foo"].Image.Name)
			}

			if tt.dev.Name != loaded.Dev["loaded"].Name {
				t.Fatalf("expected %s got %s", tt.dev.Name, loaded.Dev["foo"].Name)
			}

		})
	}
	name := "demo-deployment"
	def, err := LoadManifestOrDefault("/tmp/bad-path", name)
	if err != nil {
		t.Fatal("default dev was not returned")
	}

	if def.Dev[name].Name != name {
		t.Errorf("expected %s, got %s", name, def.Dev[name].Name)
	}

	_, err = LoadManifestOrDefault("/tmp/bad-path", "")
	if err == nil {
		t.Error("expected error with empty deployment name")
	}
}

func Test_ParseURL(t *testing.T) {
	tests := []struct {
		name    string
		u       string
		want    string
		wantErr bool
	}{
		{
			name: "full",
			u:    "https://okteto.cloud.okteto.net",
			want: "https://okteto.cloud.okteto.net",
		},
		{
			name: "no-protocol",
			u:    "okteto.cloud.okteto.net",
			want: "https://okteto.cloud.okteto.net",
		},
		{
			name: "trim-slash",
			u:    "https://okteto.cloud.okteto.net/",
			want: "https://okteto.cloud.okteto.net",
		},
		{
			name: "ip",
			u:    "https://192.168.0.1",
			want: "https://192.168.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_CheckIfDirectory(t *testing.T) {
	tests := []struct {
		name string
		path string
		want error
	}{
		{
			name: "directory",
			path: ".",
			want: nil,
		},
		{
			name: "file",
			path: "dev.go",
			want: fmt.Errorf("'dev.go' is not a directory"),
		},
		{
			name: "file",
			path: "no.go",
			want: fmt.Errorf("'no.go' does not exist"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckIfDirectory(tt.path)
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil {
				t.Errorf("CheckIfDirectory(%s) = %s, want %s", tt.path, got, tt.want)
			}
			if got.Error() != tt.want.Error() {
				t.Errorf("CheckIfDirectory(%s) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}

func Test_CheckIfRegularFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want error
	}{
		{
			name: "file",
			path: "dev.go",
			want: nil,
		},
		{
			name: "directory",
			path: ".",
			want: fmt.Errorf("'.' is not a regular file"),
		},
		{
			name: "file",
			path: "no.go",
			want: fmt.Errorf("'no.go' does not exist"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckIfRegularFile(tt.path)
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil {
				t.Errorf("CheckIfRegularFile(%s) = %s, want %s", tt.path, got, tt.want)
			}
			if got.Error() != tt.want.Error() {
				t.Errorf("CheckIfRegularFile(%s) = %s, want %s", tt.path, got, tt.want)
			}
		})
	}
}

func Test_GetDevFromManifest(t *testing.T) {
	wrongDevName := "not-test"
	tests := []struct {
		name     string
		manifest *model.Manifest
		devName  string
		dev      *model.Dev
		err      error
	}{
		{
			name:     "manifest has no dev section",
			manifest: &model.Manifest{},
			devName:  "",
			dev:      nil,
			err:      oktetoErrors.ErrManifestNoDevSection,
		},
		{
			name: "manifest has one dev section but not the one the user added",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{},
				},
			},
			devName: wrongDevName,
			dev:     nil,
			err:     fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, wrongDevName),
		},
		{
			name: "manifest has several dev section user introduces wrong one",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test":   &model.Dev{},
					"test-2": &model.Dev{},
				},
			},
			devName: wrongDevName,
			dev:     nil,
			err:     fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, wrongDevName),
		},
		{
			name: "manifest has several dev section user introduces correct one",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test":   &model.Dev{},
					"test-2": &model.Dev{},
				},
			},
			devName: "test",
			dev:     &model.Dev{},
			err:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := GetDevFromManifest(tt.manifest, tt.devName)
			assert.Equal(t, tt.dev, dev)
			if tt.err != nil {
				assert.Equal(t, tt.err.Error(), err.Error())
			}
		})
	}
}
func Test_GetDevDetach(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sync")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tests := []struct {
		name  string
		input *model.Manifest
		devs  []string
		want  *model.Dev
	}{
		{
			name: "stack without devs",
			devs: []string{},
			input: &model.Manifest{
				Type: model.StackType,
				Deploy: &model.DeployInfo{
					ComposeSection: &model.ComposeSectionInfo{
						Stack: &model.Stack{
							Services: map[string]*model.Service{
								"test": {
									Command: model.Command{Values: []string{"test"}},
									VolumeMounts: []model.StackVolume{
										{
											LocalPath:  tmpDir,
											RemotePath: "/app",
										},
									},
									Ports: []model.Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
										{
											HostPort:      0,
											ContainerPort: 5005,
										},
									},
								},
								"test-1": {
									Command: model.Command{Values: []string{"test"}},
									VolumeMounts: []model.StackVolume{
										{
											LocalPath:  tmpDir,
											RemotePath: "/app",
										},
									},
									Ports: []model.Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
										{
											HostPort:      0,
											ContainerPort: 5005,
										},
									},
								},
							},
						},
					},
				},
			},
			want: &model.Dev{
				Name:        detachModePodName,
				Namespace:   okteto.Context().Namespace,
				Context:     okteto.Context().Name,
				Autocreate:  true,
				Environment: model.Environment{},
				Volumes:     []model.Volume{},
				Secrets:     []model.Secret{},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  tmpDir,
							RemotePath: "/test/app",
						},
						{
							LocalPath:  tmpDir,
							RemotePath: "/test-1/app",
						},
					},
				},
				Forward: []model.Forward{
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test",
					},
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test-1",
					},
				},
				Services: []*model.Dev{
					{
						Name:    "test",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
					},
					{
						Name:    "test-1",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
					},
				},
			},
		},
		{
			name: "stack without devs",
			devs: []string{"test"},
			input: &model.Manifest{
				Type: model.StackType,
				Deploy: &model.DeployInfo{
					ComposeSection: &model.ComposeSectionInfo{
						Stack: &model.Stack{
							Services: map[string]*model.Service{
								"test": {
									Command: model.Command{Values: []string{"test"}},
									VolumeMounts: []model.StackVolume{
										{
											LocalPath:  tmpDir,
											RemotePath: "/app",
										},
									},
									Ports: []model.Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
										{
											HostPort:      0,
											ContainerPort: 5005,
										},
									},
								},
								"test-1": {
									Command: model.Command{Values: []string{"test"}},
									VolumeMounts: []model.StackVolume{
										{
											LocalPath:  tmpDir,
											RemotePath: "/app",
										},
									},
									Ports: []model.Port{
										{
											HostPort:      8080,
											ContainerPort: 8080,
										},
										{
											HostPort:      0,
											ContainerPort: 5005,
										},
									},
								},
							},
						},
					},
				},
			},
			want: &model.Dev{
				Name:        detachModePodName,
				Namespace:   okteto.Context().Namespace,
				Context:     okteto.Context().Name,
				Autocreate:  true,
				Environment: model.Environment{},
				Volumes:     []model.Volume{},
				Secrets:     []model.Secret{},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  tmpDir,
							RemotePath: "/test/app",
						},
					},
				},
				Forward: []model.Forward{
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test",
					},
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test-1",
					},
				},
				Services: []*model.Dev{
					{
						Name:    "test",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
					},
				},
			},
		},
		{
			name: "devs without dev",
			devs: []string{},
			input: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": {
						Name:    "test",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
						Forward: []model.Forward{
							{
								Local:  8080,
								Remote: 8080,
							},
						},
					},
					"test-1": {
						Name:    "test-1",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
						Forward: []model.Forward{
							{
								Local:  8080,
								Remote: 8080,
							},
						},
					},
				},
			},
			want: &model.Dev{
				Name:        detachModePodName,
				Namespace:   okteto.Context().Namespace,
				Context:     okteto.Context().Name,
				Autocreate:  true,
				Environment: model.Environment{},
				Volumes:     []model.Volume{},
				Secrets:     []model.Secret{},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  tmpDir,
							RemotePath: "/test/app",
						},
						{
							LocalPath:  tmpDir,
							RemotePath: "/test-1/app",
						},
					},
				},
				Forward: []model.Forward{
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test",
					},
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test-1",
					},
				},
				Services: []*model.Dev{
					{
						Name:    "test",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
					},
					{
						Name:    "test-1",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
					},
				},
			},
		},
		{
			name: "devs with dev",
			devs: []string{"test"},
			input: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": {
						Name:    "test",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
						Forward: []model.Forward{
							{
								Local:  8080,
								Remote: 8080,
							},
						},
					},
					"test-1": {
						Name:    "test-1",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
						Forward: []model.Forward{
							{
								Local:  8080,
								Remote: 8080,
							},
						},
					},
				},
			},
			want: &model.Dev{
				Name:        detachModePodName,
				Namespace:   okteto.Context().Namespace,
				Context:     okteto.Context().Name,
				Autocreate:  true,
				Environment: model.Environment{},
				Volumes:     []model.Volume{},
				Secrets:     []model.Secret{},
				PersistentVolumeInfo: &model.PersistentVolumeInfo{
					Enabled: true,
				},
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  tmpDir,
							RemotePath: "/test/app",
						},
					},
				},
				Forward: []model.Forward{
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test",
					},
					{
						Local:       8080,
						Remote:      8080,
						Service:     true,
						ServiceName: "test-1",
					},
				},
				Services: []*model.Dev{
					{
						Name:    "test",
						Command: model.Command{Values: []string{"test"}},
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  tmpDir,
									RemotePath: "/app",
								},
							},
						},
						PersistentVolumeInfo: &model.PersistentVolumeInfo{
							Enabled: true,
						},
						Volumes:     []model.Volume{},
						Environment: nil,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want.SetDefaults()
			for _, d := range tt.want.Services {
				d.SetDefaults()
			}
			for _, d := range tt.input.Dev {
				d.SetDefaults()
			}

			d, err := GetDevDetachMode(tt.input, tt.devs)
			assert.NoError(t, err)

			assert.Len(t, d.Services, len(tt.want.Services))
			assert.Len(t, d.Sync.Folders, len(tt.want.Sync.Folders))
			assert.Len(t, d.Forward, len(tt.want.Forward))

			for _, testDev := range d.Services {
				if d.Name != "test" {
					continue
				}
				assert.Equal(t, tt.want.Services[0].Sync.Folders, testDev.Sync.Folders)
				assert.Equal(t, tt.want.Services[0].Forward, testDev.Forward)
			}
		})
	}
}
