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

package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

func Test_loadDevOrDefault(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := LoadDevOrDefault("/tmp/a-path", tt.deployment, "namespace", "context")
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error when loading")
				}

				if !errors.IsNotExist(err) {
					t.Fatalf("expected not found got: %s", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if def.Name != tt.deployment {
				t.Errorf("expected default name, got %s", def.Name)
			}

			if tt.dev == nil {
				return
			}

			f, err := ioutil.TempFile("", "")
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			defer os.Remove(f.Name())

			if err := tt.dev.Save(f.Name()); err != nil {
				t.Fatal(err)
			}

			loaded, err := LoadDevOrDefault(f.Name(), "foo", "namespace", "context")
			if err != nil {
				t.Fatalf("unexpected error when loading existing manifest: %s", err.Error())
			}

			if tt.dev.Image.Name != loaded.Image.Name {
				t.Fatalf("expected %s got %s", tt.dev.Image.Name, loaded.Image.Name)
			}

			if tt.dev.Name != loaded.Name {
				t.Fatalf("expected %s got %s", tt.dev.Name, loaded.Name)
			}

		})
	}
	name := "demo-deployment"
	def, err := LoadDevOrDefault("/tmp/bad-path", name, "namespace", "context")
	if err != nil {
		t.Fatal("default dev was not returned")
	}

	if def.Name != name {
		t.Errorf("expected %s, got %s", name, def.Name)
	}

	_, err = LoadDevOrDefault("/tmp/bad-path", "", "namespace", "context")
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
