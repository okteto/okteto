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
	"errors"
	"fmt"
	"os"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/prompt"
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
			name: "manifest has one dev section and devName is empty",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{
						Name: "test",
					},
				},
			},
			devName: "",
			dev: &model.Dev{
				Name: "test",
			},
			err: nil,
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
					"test": &model.Dev{
						Name: "test",
					},
					"test-2": &model.Dev{
						Name: "test-2",
					},
				},
			},
			devName: "test",
			dev: &model.Dev{
				Name: "test",
			},
			err: nil,
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

type FakeSelector struct {
	selected string
	err      error
}

func NewFakeSelector(selected string, err error) *FakeSelector {
	return &FakeSelector{
		selected: selected,
		err:      err,
	}
}

func (fs *FakeSelector) Ask() (string, error) {
	return fs.selected, fs.err
}

func Test_SelectDevFromManifest(t *testing.T) {
	tests := []struct {
		name        string
		selector    *FakeSelector
		manifest    *model.Manifest
		expectedDev *model.Dev
		expectedErr error
	}{
		{
			name:        "selector-returns-error",
			selector:    NewFakeSelector("", errors.New("error")),
			manifest:    &model.Manifest{},
			expectedErr: errors.New("error"),
		},
		{
			name:     "selector-returns-devname",
			selector: NewFakeSelector("test", nil),
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{
						Name:            "test",
						ImagePullPolicy: "Always",
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  "/",
									RemotePath: "/remote",
								},
							},
						},
						SSHServerPort: 80,
						Image:         &model.BuildInfo{},
					},
					"test-2": &model.Dev{
						Name:            "test-2",
						ImagePullPolicy: "Always",
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  "/",
									RemotePath: "/remote",
								},
							},
						},
						SSHServerPort: 80,
						Image:         &model.BuildInfo{},
					},
				},
			},
			expectedErr: nil,
			expectedDev: &model.Dev{
				Name:            "test",
				ImagePullPolicy: "Always",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  "/",
							RemotePath: "/remote",
						},
					},
				},
				SSHServerPort: 80,
				Image:         &model.BuildInfo{},
			},
		},
		{
			name:     "selector-returns-invalid-error",
			selector: NewFakeSelector("test", nil),
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"test": &model.Dev{
						Name: "test",
					},
					"test-2": &model.Dev{
						Name:            "test-2",
						ImagePullPolicy: "Always",
						Sync: model.Sync{
							Folders: []model.SyncFolder{
								{
									LocalPath:  "/",
									RemotePath: "/remote",
								},
							},
						},
						SSHServerPort: 80,
						Image:         &model.BuildInfo{},
					},
				},
			},
			expectedErr: errors.New("supported values for 'imagePullPolicy' are: 'Always', 'IfNotPresent' or 'Never'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := SelectDevFromManifest(tt.manifest, tt.selector)
			assert.EqualValues(t, tt.expectedErr, err)
			assert.EqualValues(t, tt.expectedDev, res)
		})
	}
}

func Test_getItemsForDevSelector(t *testing.T) {
	tests := []struct {
		name     string
		devs     model.ManifestDevs
		expected []prompt.SelectorItem
	}{
		{
			"empty-devs",
			model.ManifestDevs{},
			[]prompt.SelectorItem{},
		},
		{
			"single-devs",
			model.ManifestDevs{
				"test": &model.Dev{},
			},
			[]prompt.SelectorItem{
				{
					Name:   "test",
					Label:  "test",
					Enable: true,
				},
			},
		},
		{
			"multiple-devs",
			model.ManifestDevs{
				"b": &model.Dev{},
				"c": &model.Dev{},
				"a": &model.Dev{},
			},
			[]prompt.SelectorItem{
				{
					Name:   "a",
					Label:  "a",
					Enable: true,
				},
				{
					Name:   "b",
					Label:  "b",
					Enable: true,
				},
				{
					Name:   "c",
					Label:  "c",
					Enable: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			items := GetItemsForDevSelector(tt.devs)
			assert.EqualValues(t, tt.expected, items)
		})
	}
}
