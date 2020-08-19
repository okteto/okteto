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
	"testing"
)

func Test_HasLocalVolumes(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		expected bool
	}{
		{
			name: "false",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "/etc1",
					},
					{
						RemotePath: "/etc2",
					},
				},
			},
			expected: false,
		},
		{
			name: "true",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "/etc1",
					},
					{
						LocalPath:  "/etc",
						RemotePath: "/etc2",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dev.HasLocalVolumes()
			if result != tt.expected {
				t.Errorf("test '%s' got '%t' instead of '%t'", tt.name, result, tt.expected)
			}
		})
	}
}

func Test_HasRemoteVolumes(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		expected bool
	}{
		{
			name: "false",
			dev: &Dev{
				Volumes: []Volume{
					{
						LocalPath:  "/etc1",
						RemotePath: "/etc1",
					},
					{
						LocalPath:  "/etc2",
						RemotePath: "/etc2",
					},
				},
			},
			expected: false,
		},
		{
			name: "true",
			dev: &Dev{
				Volumes: []Volume{
					{
						RemotePath: "/etc1",
					},
					{
						LocalPath:  "/etc2",
						RemotePath: "/etc2",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dev.HasRemoteVolumes()
			if result != tt.expected {
				t.Errorf("test '%s' got '%t' instead of '%t'", tt.name, result, tt.expected)
			}
		})
	}
}

func Test_IsSyncFolder(t *testing.T) {
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
				Volumes: []Volume{
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
				Volumes: []Volume{
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
			expected: true,
			wantErr:  false,
		},
		{
			name: "subpath",
			dev: &Dev{
				Volumes: []Volume{
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
			expected: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.dev.IsSyncFolder(tt.path)
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
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
					{
						LocalPath:  "/local/subpath",
						RemotePath: "/subpath",
					},
					{
						RemotePath: "/cache",
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
						LocalPath:  "/local",
						RemotePath: "/remote",
					},
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
				Volumes: []Volume{
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
				Volumes: []Volume{
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
						LocalPath:  "src",
						RemotePath: "/src",
					},
					{
						RemotePath: "/remote",
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
			name: "duplicated",
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
				Volumes: []Volume{
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
				Volumes: []Volume{
					{
						LocalPath:  "/src1",
						RemotePath: "/remote1",
					},
				},
				Services: []*Dev{
					{
						Volumes: []Volume{
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
