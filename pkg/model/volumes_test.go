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
	"reflect"
	"runtime"
	"testing"
)

func TestDev_translateDeprecatedVolumeFields(t *testing.T) {
	tests := []struct {
		name    string
		dev     *Dev
		result  *Dev
		wantErr bool
	}{
		{
			name: "workdir",
			dev: &Dev{
				WorkDir: "/workdir",
				Volumes: []Volume{},
				Syncs:   []Sync{},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/workdir",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mountpath",
			dev: &Dev{
				MountPath: "/mountpath",
				Volumes:   []Volume{},
				Syncs:     []Sync{},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/mountpath",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workdir-and-mountpath",
			dev: &Dev{
				WorkDir:   "/workdir",
				MountPath: "/mountpath",
				Volumes:   []Volume{},
				Syncs:     []Sync{},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/mountpath",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workdir-syncs",
			dev: &Dev{
				WorkDir: "/workdir",
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mountpath-syncs",
			dev: &Dev{
				MountPath: "/mountpath",
				Volumes:   []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
					{
						LocalPath:  ".",
						RemotePath: "/mountpath",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workdir-and-mountpath-syncs",
			dev: &Dev{
				WorkDir:   "/workdir",
				MountPath: "/mountpath",
				Volumes:   []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
					{
						LocalPath:  ".",
						RemotePath: "/mountpath",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "volumes-to-syncs",
			dev: &Dev{
				WorkDir:   "/workdir",
				MountPath: "/mountpath",
				Volumes: []Volume{
					{
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
				},
				Syncs: []Sync{},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/mountpath",
					},
					{
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-workdir",
			dev: &Dev{
				WorkDir: "/workdir1",
				Volumes: []Volume{},
				Syncs:   []Sync{},
				Services: []*Dev{
					{
						WorkDir: "/workdir2",
						Volumes: []Volume{},
						Syncs:   []Sync{},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/workdir1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  ".",
								RemotePath: "/workdir2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-workdir-subpath",
			dev: &Dev{
				WorkDir: "/workdir1",
				Volumes: []Volume{},
				Syncs:   []Sync{},
				Services: []*Dev{
					{
						WorkDir: "/workdir2",
						SubPath: "subpath",
						Volumes: []Volume{},
						Syncs:   []Sync{},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/workdir1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "subpath",
								RemotePath: "/workdir2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-mountpath",
			dev: &Dev{
				MountPath: "/mountpath1",
				Volumes:   []Volume{},
				Syncs:     []Sync{},
				Services: []*Dev{
					{
						MountPath: "/mountpath2",
						Volumes:   []Volume{},
						Syncs:     []Sync{},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/mountpath1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  ".",
								RemotePath: "/mountpath2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-mountpath-subpath",
			dev: &Dev{
				MountPath: "/mountpath1",
				Volumes:   []Volume{},
				Syncs:     []Sync{},
				Services: []*Dev{
					{
						MountPath: "/mountpath2",
						SubPath:   "subpath",
						Volumes:   []Volume{},
						Syncs:     []Sync{},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/mountpath1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "subpath",
								RemotePath: "/mountpath2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-workdir-error",
			dev: &Dev{
				WorkDir: "/workdir1",
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
				Services: []*Dev{
					{
						WorkDir: "/workdir1",
					},
				},
			},
			result:  nil,
			wantErr: true,
		},
		{
			name: "services-mountpath-error",
			dev: &Dev{
				WorkDir: "/mountpath1",
				Syncs: []Sync{
					{
						LocalPath:  "local",
						RemotePath: "remote",
					},
				},
				Services: []*Dev{
					{
						MountPath: "/mountpath2",
					},
				},
			},
			result:  nil,
			wantErr: true,
		},
		{
			name: "services-workdir-syncs",
			dev: &Dev{
				WorkDir: "/workdir1",
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local1",
						RemotePath: "remote1",
					},
				},
				Services: []*Dev{
					{
						WorkDir: "/workdir2",
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "local2",
								RemotePath: "remote2",
							},
						},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local1",
						RemotePath: "remote1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "local2",
								RemotePath: "remote2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-mountpath-syncs",
			dev: &Dev{
				MountPath: "/mountpath1",
				Volumes:   []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local1",
						RemotePath: "remote1",
					},
				},
				Services: []*Dev{
					{
						MountPath: "/mountpath2",
						Volumes:   []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "local2",
								RemotePath: "remote2",
							},
						},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local1",
						RemotePath: "remote1",
					},
					{
						LocalPath:  ".",
						RemotePath: "/mountpath1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "local2",
								RemotePath: "remote2",
							},
							{
								LocalPath:  ".",
								RemotePath: "/mountpath2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-workdir-and-mountpath-syncs",
			dev: &Dev{
				WorkDir:   "/workdir1",
				MountPath: "/mountpath1",
				Volumes:   []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local1",
						RemotePath: "remote1",
					},
				},
				Services: []*Dev{
					{
						WorkDir:   "/workdir2",
						MountPath: "/mountpath2",
						Volumes:   []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "local2",
								RemotePath: "remote2",
							},
						},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  "local1",
						RemotePath: "remote1",
					},
					{
						LocalPath:  ".",
						RemotePath: "/mountpath1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  "local2",
								RemotePath: "remote2",
							},
							{
								LocalPath:  ".",
								RemotePath: "/mountpath2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-volumes-to-syncs",
			dev: &Dev{
				WorkDir: "/workdir1",
				Volumes: []Volume{
					{
						LocalPath:  "/local1",
						RemotePath: "/remote1",
					},
				},
				Syncs: []Sync{},
				Services: []*Dev{
					{
						WorkDir: "/workdir2",
						Volumes: []Volume{
							{
								LocalPath:  "/local2",
								RemotePath: "/remote2",
							},
						},
						Syncs: []Sync{},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Syncs: []Sync{
					{
						LocalPath:  ".",
						RemotePath: "/workdir1",
					},
					{
						LocalPath:  "/local1",
						RemotePath: "/remote1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Syncs: []Sync{
							{
								LocalPath:  ".",
								RemotePath: "/workdir2",
							},
							{
								LocalPath:  "/local2",
								RemotePath: "/remote2",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dev.translateDeprecatedVolumeFields()
			if tt.wantErr {
				if err == nil {
					t.Errorf("test '%s': error was expected", tt.name)
				}
				return
			}
			if err != nil {
				t.Errorf("test '%s': unexpected error: %s", tt.name, err.Error())
			}
			if !reflect.DeepEqual(tt.dev.Volumes, tt.result.Volumes) {
				t.Errorf("test '%s': expected main volumes: %v, actual: %v", tt.name, tt.result.Volumes, tt.dev.Volumes)
			}
			if !reflect.DeepEqual(tt.dev.Syncs, tt.result.Syncs) {
				t.Errorf("test '%s': expected main syncs: %v, actual: %v", tt.name, tt.result.Syncs, tt.dev.Syncs)
			}
			for i, s := range tt.dev.Services {
				if !reflect.DeepEqual(s.Volumes, tt.result.Services[i].Volumes) {
					t.Errorf("test '%s': expected service volumes: %v, actual: %v", tt.name, tt.result.Services[i].Volumes, s.Volumes)
				}
				if !reflect.DeepEqual(s.Syncs, tt.result.Services[i].Syncs) {
					t.Errorf("test '%s': expected service syncs: %v, actual: %v", tt.name, tt.result.Services[i].Syncs, s.Syncs)
				}
			}
		})
	}
}

func Test_IsSubPathFolder(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		path     string
		expected bool
		wantErr  bool
	}{
		{
			name: "not-found",
			dev: &Dev{
				Syncs: []Sync{
					{
						LocalPath:  "/etc",
						RemotePath: "/etc",
					},
				},
			},
			path:     "/var",
			expected: false,
			wantErr:  true,
		},
		{
			name: "root",
			dev: &Dev{
				Syncs: []Sync{
					{
						LocalPath:  "/etc1",
						RemotePath: "/etc1",
					},
					{
						LocalPath:  "/var",
						RemotePath: "/var",
					},
					{
						LocalPath:  "/etc2",
						RemotePath: "/etc2",
					},
				},
			},
			path:     "/var",
			expected: false,
			wantErr:  false,
		},
		{
			name: "subpath",
			dev: &Dev{
				Syncs: []Sync{
					{
						LocalPath:  "/etc1",
						RemotePath: "/etc1",
					},
					{
						LocalPath:  "/var",
						RemotePath: "/var",
					},
					{
						LocalPath:  "/var/foo",
						RemotePath: "/var/foo",
					},
				},
			},
			path:     "/var/foo/aaa",
			expected: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.dev.IsSubPathFolder(tt.path)
			if tt.wantErr && err != nil {
				return
			}
			if err != nil {
				t.Errorf("'%s' got unexpected error: %s", tt.name, err.Error())
			}
			if result != tt.expected {
				t.Errorf("'%s' got '%t' expected '%t'", tt.name, result, tt.expected)
			}
		})
	}
}

func Test_getDataSubPath(t *testing.T) {
	var tests = []struct {
		name   string
		path   string
		result string
	}{
		{
			name:   "single",
			path:   "/var",
			result: "data/var",
		},
		{
			name:   "double",
			path:   "/var/okteto",
			result: "data/var/okteto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDataSubPath(tt.path)
			if result != tt.result {
				t.Errorf("'%s' got '%s' expected '%s'", tt.name, result, tt.result)
			}
		})
	}
}

func Test_getSourceSubPath(t *testing.T) {
	var tests = []struct {
		name    string
		path    string
		linux   bool
		windows bool
		result  string
	}{
		{
			name:    "relative",
			path:    "code/func",
			linux:   true,
			windows: true,
			result:  "src/code/func",
		},
		{
			name:    "linux",
			path:    "/code/func",
			linux:   true,
			windows: false,
			result:  "src/code/func",
		},
		{
			name:    "windows",
			path:    "c:\\code\\func",
			linux:   false,
			windows: true,
			result:  "src/code/func",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.linux && (runtime.GOOS == "linux" || runtime.GOOS == "darwin")) || (tt.windows && runtime.GOOS == "windows") {
				result := getSourceSubPath(tt.path)
				if result != tt.result {
					t.Errorf("'%s' got '%s' for '%s', expected '%s'", tt.name, result, runtime.GOOS, tt.result)
				}
			}
		})
	}
}

func Test_validatePersistentVolume(t *testing.T) {
	var tests = []struct {
		name    string
		dev     *Dev
		wantErr bool
	}{
		{
			name: "enabled",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "/cache",
					},
				},
				Syncs: []Sync{
					{
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
					{
						LocalPath:  "/local/subpath",
						RemotePath: "/subpath",
					},
				},
				Services: []*Dev{
					{
						Name: "worker",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "not-enabled-and-services",
			dev: &Dev{
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: false,
				},
				Services: []*Dev{
					{
						Name: "worker",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "not-enabled-and-data-volumes",
			dev: &Dev{
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: false,
				},
				Volumes: []Volume{
					{
						RemotePath: "/cache",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "not-enabled-and-subpath",
			dev: &Dev{
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: false,
				},
				Syncs: []Sync{
					{
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
					{
						LocalPath:  "/local/subpath",
						RemotePath: "/subpath",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ok-not-enabled",
			dev: &Dev{
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: false,
				},
				Syncs: []Sync{
					{
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dev.validatePersistentVolume()
			if err == nil && tt.wantErr {
				t.Errorf("'%s' did get an expected error", tt.name)
			}
			if err != nil && !tt.wantErr {
				t.Errorf("'%s' got unexpected error: %s", tt.name, err.Error())
			}
		})
	}
}

func Test_validateVolumes(t *testing.T) {
	var tests = []struct {
		name    string
		dev     *Dev
		wantErr bool
	}{
		{
			name: "ok",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "/remote",
					},
				},
				Syncs: []Sync{
					{
						LocalPath:  "src",
						RemotePath: "/src",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "relative-remote",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "remote",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicated-volume",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "/remote",
					},
					{
						RemotePath: "/remote",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicated-sync",
			dev: &Dev{
				Syncs: []Sync{
					{
						LocalPath:  "src",
						RemotePath: "/remote1",
					},
					{
						LocalPath:  "src",
						RemotePath: "/remote2",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "wrong-service-sync-folder",
			dev: &Dev{
				Syncs: []Sync{
					{
						LocalPath:  "/src1",
						RemotePath: "/remote1",
					},
				},
				Services: []*Dev{
					{
						Syncs: []Sync{
							{
								LocalPath:  "/src2",
								RemotePath: "/remote2",
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dev.validateVolumes(nil)
			if err != nil && tt.wantErr {
				return
			}
			if err != nil && !tt.wantErr {
				t.Errorf("'%s' got unexpected error: %s", tt.name, err.Error())
			}
			for _, s := range tt.dev.Services {
				err := s.validateVolumes(tt.dev)
				if err != nil && tt.wantErr {
					return
				}
				if err != nil && !tt.wantErr {
					t.Errorf("'%s' got unexpected error: %s", tt.name, err.Error())
				}
			}
		})
	}
}
