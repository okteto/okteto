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
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

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
				Icon:          "",
				Dev:           model.ManifestDevs{},
				Type:          model.OktetoManifestType,
				Destroy:       &model.DestroyInfo{},
				Deploy:        &model.DeployInfo{},
				GlobalForward: []forward.GlobalForward{},
				Test:          model.ManifestTests{},
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
