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

package context

import (
	"os"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func Test_addKubernetesContext(t *testing.T) {
	var tests = []struct {
		cfg          *clientcmdapi.Config
		ctxResource  *model.ContextResource
		currentStore *okteto.ContextStore
		wantStore    *okteto.ContextStore
		name         string
		wantError    bool
	}{
		{
			name:        "nil-cfg",
			ctxResource: &model.ContextResource{Context: "context"},
			wantError:   true,
		},
		{
			name: "not-found",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{},
			},
			ctxResource: &model.ContextResource{Context: "context"},
			wantError:   true,
		},
		{
			name: "found-and-ctxresource-namespace",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{"context": {Namespace: "n-cfg"}},
			},
			ctxResource: &model.ContextResource{Context: "context", Namespace: "n-ctx"},
			currentStore: &okteto.ContextStore{
				CurrentContext: "",
				Contexts:       map[string]*okteto.Context{},
			},
			wantStore: &okteto.ContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.Context{
					"context": {Name: "context", Namespace: "n-ctx", Analytics: true},
				},
			},
			wantError: false,
		},
		{
			name: "found-and-cfg-namespace",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{"context": {Namespace: "n-cfg"}},
			},
			ctxResource: &model.ContextResource{Context: "context"},
			currentStore: &okteto.ContextStore{
				CurrentContext: "",
				Contexts:       map[string]*okteto.Context{},
			},
			wantStore: &okteto.ContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.Context{
					"context": {Name: "context", Namespace: "n-cfg", Analytics: true},
				},
			},
			wantError: false,
		},
		{
			name: "found-and-default-namespace",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{"context": {}},
			},
			ctxResource: &model.ContextResource{Context: "context"},
			currentStore: &okteto.ContextStore{
				CurrentContext: "",
				Contexts:       map[string]*okteto.Context{},
			},
			wantStore: &okteto.ContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.Context{
					"context": {Name: "context", Namespace: "default", Analytics: true},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.currentStore
			err := addKubernetesContext(tt.cfg, tt.ctxResource)
			if err != nil && !tt.wantError {
				t.Errorf("Test '%s' failed: %+v", tt.name, err)
			}
			if err == nil && tt.wantError {
				t.Errorf("Test '%s' didn't failed", tt.name)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.wantStore, okteto.CurrentStore) {
				t.Errorf("Test '%s' failed: %+v", tt.name, okteto.CurrentStore)
			}
		})
	}
}

func Test_GetManifestV2(t *testing.T) {
	tests := []struct {
		expectedManifest *model.Manifest
		name             string
		file             string
		manifestYAML     []byte
		expectedErr      bool
	}{
		{
			name:        "file-is-defined-option",
			file:        "file",
			expectedErr: false,
			manifestYAML: []byte(`
namespace: test-namespace
context: manifest-context
build:
  service:
    image: defined-tag-image
    context: ./service
    target: build
    dockerfile: custom-dockerfile
    args:
      KEY1: Value1
      KEY2: Value2
    cache_from:
      - cache-image-1
      - cache-image-2
dependencies:
  one: https://repo.url`),
			expectedManifest: &model.Manifest{
				IsV2:      true,
				Namespace: "test-namespace",
				Context:   "manifest-context",
				Build: build.ManifestBuild{
					"service": {
						Name:       "",
						Target:     "build",
						Context:    "./service",
						Dockerfile: "custom-dockerfile",
						Image:      "defined-tag-image",
						Args: []build.Arg{
							{
								Name: "KEY1", Value: "Value1",
							},
							{
								Name: "KEY2", Value: "Value2",
							},
						},
						CacheFrom: []string{"cache-image-1", "cache-image-2"},
					},
				},
				Icon:    "",
				Dev:     model.ManifestDevs{},
				Type:    model.OktetoManifestType,
				Destroy: &model.DestroyInfo{},
				Dependencies: deps.ManifestSection{
					"one": &deps.Dependency{
						Repository: "https://repo.url",
					},
				},
				External: externalresource.Section{},
				Fs:       afero.NewOsFs(),
			},
		},
		{
			name:        "manifest-path-not-found",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tmpFile, err := os.CreateTemp("", tt.file)
			if err != nil {
				t.Fatalf("failed to create dynamic manifest file: %s", err.Error())
			}
			if err := os.WriteFile(tmpFile.Name(), tt.manifestYAML, 0600); err != nil {
				t.Fatalf("failed to write manifest file: %s", err.Error())
			}
			defer os.RemoveAll(tmpFile.Name())

			filename := tmpFile.Name()
			if tt.file == "" {
				filename = ""
			}

			m, err := model.GetManifestV2(filename, afero.NewMemMapFs())
			if tt.expectedErr {
				assert.NotNil(t, err)
			} else {
				m.Manifest = nil
				tt.expectedManifest.ManifestPath = filename
				assert.EqualValues(t, tt.expectedManifest, m)
			}

		})
	}
}

func Test_GetCtxResource(t *testing.T) {
	tests := []struct {
		expectedErr         error
		expectedCtxResource *model.ContextResource
		name                string
		manifestName        string
		manifestYAML        []byte
	}{
		{
			name:         "valid manifest returns a initialized ctx resource",
			expectedErr:  nil,
			manifestName: "okteto.yml",
			manifestYAML: []byte(`
namespace: test-namespace
context: manifest-context
`),
			expectedCtxResource: &model.ContextResource{
				Namespace: "test-namespace",
				Context:   "manifest-context",
			},
		},
		{
			name:                "no valid manifest returns a zero value ctx resource",
			expectedErr:         nil,
			manifestName:        "",
			expectedCtxResource: &model.ContextResource{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestPath := ""
			if tt.manifestName != "" {
				tmpFile, err := os.CreateTemp("", tt.manifestName)
				if err != nil {
					t.Fatalf("failed to create dynamic manifest file: %s", err.Error())
				}
				if err := os.WriteFile(tmpFile.Name(), tt.manifestYAML, 0600); err != nil {
					t.Fatalf("failed to write manifest file: %s", err.Error())
				}
				defer os.RemoveAll(tmpFile.Name())

				manifestPath = tmpFile.Name()
			}

			ctxResource, err := getCtxResource(manifestPath)
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.EqualValues(t, ctxResource, tt.expectedCtxResource)
		})
	}
}
