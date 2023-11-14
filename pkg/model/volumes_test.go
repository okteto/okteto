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
	"reflect"
	"runtime"
	"testing"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDev_translateDeprecatedVolumeFields(t *testing.T) {
	tests := []struct {
		dev     *Dev
		result  *Dev
		name    string
		wantErr bool
	}{
		{
			name: "none",
			dev: &Dev{
				Workdir: "",
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Workdir: "/okteto",
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  ".",
							RemotePath: "/okteto",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workdir",
			dev: &Dev{
				Workdir: "/workdir",
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  ".",
							RemotePath: "/workdir",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workdir-syncs",
			dev: &Dev{
				Workdir: "/workdir",
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "local",
							RemotePath: "remote",
						},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "local",
							RemotePath: "remote",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "services-workdir-error",
			dev: &Dev{
				Workdir: "/workdir1",
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "local",
							RemotePath: "remote",
						},
					},
				},
				Services: []*Dev{
					{
						Workdir: "/workdir1",
					},
				},
			},
			result:  nil,
			wantErr: true,
		},
		{
			name: "services-workdir-syncs",
			dev: &Dev{
				Workdir: "/workdir1",
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "local1",
							RemotePath: "remote1",
						},
					},
				},
				Services: []*Dev{
					{
						Workdir: "/workdir2",
						Volumes: []Volume{},
						Sync: Sync{
							Folders: []SyncFolder{
								{
									LocalPath:  "local2",
									RemotePath: "remote2",
								},
							},
						},
					},
				},
			},
			result: &Dev{
				Volumes: []Volume{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "local1",
							RemotePath: "remote1",
						},
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{},
						Sync: Sync{
							Folders: []SyncFolder{
								{
									LocalPath:  "local2",
									RemotePath: "remote2",
								},
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
			if !reflect.DeepEqual(tt.dev.Sync, tt.result.Sync) {
				t.Errorf("test '%s': expected main syncs: %v, actual: %v", tt.name, tt.result.Sync, tt.dev.Sync)
			}
			for i, s := range tt.dev.Services {
				if !reflect.DeepEqual(s.Volumes, tt.result.Services[i].Volumes) {
					t.Errorf("test '%s': expected service volumes: %v, actual: %v", tt.name, tt.result.Services[i].Volumes, s.Volumes)
				}
				if !reflect.DeepEqual(s.Sync, tt.result.Services[i].Sync) {
					t.Errorf("test '%s': expected service syncs: %v, actual: %v", tt.name, tt.result.Services[i].Sync, s.Sync)
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
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "/etc",
							RemotePath: "/etc",
						},
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
				Sync: Sync{
					Folders: []SyncFolder{
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
			},
			path:     "/var",
			expected: false,
			wantErr:  false,
		},
		{
			name: "subpath",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
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

func Test_computeParentSyncFolder(t *testing.T) {
	var tests = []struct {
		name   string
		dev    *Dev
		goos   string
		result string
	}{
		{
			name: "linux-preffix",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "/aaa/111111",
						},
						{
							LocalPath: "/aaa/111222",
						},
					},
				},
			},
			goos:   "linux",
			result: "/aaa",
		},
		{
			name: "windows-double-preffix",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "c:\\common\\aaa\\111",
						},
						{
							LocalPath: "c:\\common\\aaa\\222",
						},
					},
				},
			},
			goos:   "windows",
			result: "/common/aaa",
		},
		{
			name: "linux-root",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "/aaa/111",
						},
						{
							LocalPath: "/bbb/222",
						},
						{
							LocalPath: "/aaa/222",
						},
					},
				},
			},
			goos:   "linux",
			result: "/",
		},
		{
			name: "darwin-root",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "/aaa/111",
						},
						{
							LocalPath: "/bbb/222",
						},
						{
							LocalPath: "/aaa/222",
						},
					},
				},
			},
			goos:   "darwin",
			result: "/",
		},
		{
			name: "windows-root",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "c:\\aaa\\111",
						},
						{
							LocalPath: "c:\\bbb\\222",
						},
						{
							LocalPath: "c:\\aaa\\222",
						},
					},
				},
			},
			goos:   "windows",
			result: "/",
		},
		{
			name: "linux-relative",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "/common/aaa/111",
						},
						{
							LocalPath: "/common/bbb/222",
						},
						{
							LocalPath: "/common/aaa/222",
						},
					},
				},
			},
			goos:   "linux",
			result: "/common",
		},
		{
			name: "darwin-relative",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "/common/aaa/111",
						},
						{
							LocalPath: "/common/bbb/222",
						},
						{
							LocalPath: "/common/aaa/222",
						},
					},
				},
			},
			goos:   "darwin",
			result: "/common",
		},
		{
			name: "windows-relative",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath: "c:\\common\\aaa\\111",
						},
						{
							LocalPath: "c:\\common\\bbb\\222",
						},
						{
							LocalPath: "c:\\common\\aaa\\222",
						},
					},
				},
			},
			goos:   "windows",
			result: "/common",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.goos == runtime.GOOS {
				tt.dev.computeParentSyncFolder()
				if tt.result != tt.dev.parentSyncFolder {
					t.Errorf("'%s' got '%s' for '%s', expected '%s'", tt.name, tt.dev.parentSyncFolder, runtime.GOOS, tt.result)
				}
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
	oktetoLog.Init(logrus.DebugLevel)
	var tests = []struct {
		name   string
		dev    *Dev
		path   string
		goos   string
		result string
	}{
		{
			name:   "linux-root",
			dev:    &Dev{parentSyncFolder: "/"},
			path:   "/code/func",
			goos:   "linux",
			result: "src/code/func",
		},
		{
			name:   "darwin-root",
			dev:    &Dev{parentSyncFolder: "/"},
			path:   "/code/func",
			goos:   "darwin",
			result: "src/code/func",
		},
		{
			name:   "windows-root",
			dev:    &Dev{parentSyncFolder: "/"},
			path:   "c:\\code\\func",
			goos:   "windows",
			result: "src/code/func",
		},
		{
			name:   "linux-relative",
			dev:    &Dev{parentSyncFolder: "/code"},
			path:   "/code/func",
			goos:   "linux",
			result: "src/func",
		},
		{
			name:   "darwin-relative",
			dev:    &Dev{parentSyncFolder: "/code"},
			path:   "/code/func",
			goos:   "darwin",
			result: "src/func",
		},
		{
			name:   "windows-relative",
			dev:    &Dev{parentSyncFolder: "/code"},
			path:   "c:\\code\\func",
			goos:   "windows",
			result: "src/func",
		},
		{
			name:   "windows-non-relative",
			dev:    &Dev{parentSyncFolder: "/code"},
			path:   "c:\\test\\func",
			goos:   "windows",
			result: "test/func",
		},
		{
			name:   "linux-non-relative",
			dev:    &Dev{parentSyncFolder: "/code"},
			path:   "/test/func",
			goos:   "linux",
			result: "test/func",
		},
		{
			name:   "darwin-non-relative",
			dev:    &Dev{parentSyncFolder: "/code"},
			path:   "/test/func",
			goos:   "darwin",
			result: "test/func",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.goos == runtime.GOOS {
				result := tt.dev.getSourceSubPath(tt.path)
				if result != tt.result {
					t.Errorf("'%s' got '%s' for '%s', expected '%s'", tt.name, result, runtime.GOOS, tt.result)
				}
			}
		})
	}
}

func Test_validatePersistentVolume(t *testing.T) {
	var tests = []struct {
		dev     *Dev
		name    string
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
				Sync: Sync{
					Folders: []SyncFolder{
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
				Sync: Sync{
					Folders: []SyncFolder{
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
			},
			wantErr: true,
		},
		{
			name: "ok-not-enabled",
			dev: &Dev{
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: false,
				},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "/local",
							RemotePath: "/remote",
						},
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
		dev     *Dev
		name    string
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
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "src",
							RemotePath: "/src",
						},
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
				Sync: Sync{
					Folders: []SyncFolder{
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
			},
			wantErr: true,
		},
		{
			name: "wrong-service-sync-folder",
			dev: &Dev{
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  "/src1",
							RemotePath: "/remote1",
						},
					},
				},
				Services: []*Dev{
					{
						Sync: Sync{
							Folders: []SyncFolder{
								{
									LocalPath:  "/src2",
									RemotePath: "/remote2",
								},
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

func TestDefaultVolumeSize(t *testing.T) {

	testCases := []struct {
		context  string
		expected string
	}{
		{
			context:  "https://cloud.okteto.com",
			expected: cloudDefaultVolumeSize,
		},
		{
			context:  "https://staging.okteto.dev",
			expected: cloudDefaultVolumeSize,
		},
		{
			context:  "other",
			expected: defaultVolumeSize,
		},
	}

	for _, testCase := range testCases {
		dev := &Dev{Context: testCase.context}
		actual := dev.getDefaultPersistentVolumeSize()
		assert.Equal(t, testCase.expected, actual)
	}

}
